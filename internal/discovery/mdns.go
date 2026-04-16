package discovery

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	apFallbackURL  = "ws://192.168.4.1/ws"
	mdnsService    = "_http._tcp"
	mdnsDomain     = "local."
	mdnsTimeout    = 4 * time.Second
)

// Discover tries to find the CYD Dashboard device on the local network.
//
// Order:
//  1. mDNS browse for _http._tcp.local — returns the first host that has
//     "cyd" in its hostname (configurable via mdnsHostHint).
//  2. If not found within timeout → returns the AP fallback address.
//
// If manualURL is non-empty, it is returned immediately without scanning.
func Discover(manualURL string, mdnsHostHint string) string {
	if manualURL != "" && manualURL != "ws://192.168.4.1/ws" {
		// If the saved URL uses a .local hostname, replace it with the mDNS-resolved IP
		// so the caller never blocks on OS mDNS resolution when dialing.
		if strings.Contains(strings.ToLower(manualURL), ".local") {
			hint := mdnsHostHint
			if hint == "" {
				hint = "cyd"
			}
			if ipURL := browseOnce(hint); ipURL != "" {
				log.Printf("[Discovery] resolved .local URL → %s", ipURL)
				return ipURL
			}
		}
		log.Printf("[Discovery] using manual URL: %s", manualURL)
		return manualURL
	}

	if mdnsHostHint == "" {
		mdnsHostHint = "cyd"
	}

	log.Printf("[Discovery] searching for '%s' via mDNS (timeout %v)...", mdnsHostHint, mdnsTimeout)

	found := browseOnce(mdnsHostHint)
	if found != "" {
		log.Printf("[Discovery] found device at %s", found)
		return found
	}

	log.Printf("[Discovery] mDNS not found, falling back to AP address %s", apFallbackURL)
	return apFallbackURL
}

// browseOnce performs a single mDNS browse and returns the WebSocket URL
// of the first matching entry, or "" if nothing is found within mdnsTimeout.
func browseOnce(hostHint string) string {
	ctx, cancel := context.WithTimeout(context.Background(), mdnsTimeout)
	defer cancel()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("[Discovery] mDNS resolver error: %v", err)
		return ""
	}

	entries := make(chan *zeroconf.ServiceEntry)
	if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
		log.Printf("[Discovery] mDNS browse error: %v", err)
		return ""
	}

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return ""
			}
			host := entry.HostName
			// HostName includes trailing dot, e.g. "cyd.local."
			if len(host) > 0 && host[len(host)-1] == '.' {
				host = host[:len(host)-1]
			}
			// Match by hostname hint (case-insensitive prefix check)
			if containsInsensitive(host, hostHint) && entry.Port > 0 {
				// Prefer IP address so the caller never needs mDNS resolution when dialing
				if len(entry.AddrIPv4) > 0 {
					return fmt.Sprintf("ws://%s:%d/ws", entry.AddrIPv4[0].String(), entry.Port)
				}
				return fmt.Sprintf("ws://%s:%d/ws", host, entry.Port)
			}
		case <-ctx.Done():
			return ""
		}
	}
}

func containsInsensitive(s, sub string) bool {
	sl := len(s)
	subl := len(sub)
	if subl > sl {
		return false
	}
	for i := 0; i <= sl-subl; i++ {
		match := true
		for j := 0; j < subl; j++ {
			cs, csub := s[i+j], sub[j]
			if cs >= 'A' && cs <= 'Z' {
				cs += 32
			}
			if csub >= 'A' && csub <= 'Z' {
				csub += 32
			}
			if cs != csub {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
