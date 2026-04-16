package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"cyd-companion/internal/collector"
	"cyd-companion/internal/config"
	"cyd-companion/internal/discovery"
	"cyd-companion/internal/hid"

	"github.com/gorilla/websocket"
)

const version = "1.0.0"

// DeviceDiag holds telemetry from the CYD firmware (diag_ack).
type DeviceDiag struct {
	UptimeMs int64 `json:"uptime_ms"`
	Wifi     struct {
		Status      int    `json:"status"`
		RSSI        int    `json:"rssi"`
		IP          string `json:"ip"`
		Disconnects int    `json:"disconnects"`
		Reconnects  int    `json:"reconnects"`
	} `json:"wifi"`
	HTTP struct {
		Enter    int `json:"enter"`
		OK       int `json:"ok"`
		Heap503  int `json:"503_heap"`
		Busy503  int `json:"503_busy"`
	} `json:"http"`
	TCP struct {
		Opens          int `json:"opens"`
		Closes         int `json:"closes"`
		RejectLowHeap  int `json:"reject_lowheap"`
	} `json:"tcp"`
	GW struct {
		PingOK   int `json:"ping_ok"`
		PingFail int `json:"ping_fail"`
	} `json:"gw"`
	Heap struct {
		Free  int `json:"free"`
		Min   int `json:"min"`
		PSRAM int `json:"psram"`
	} `json:"heap"`
}

// State holds observable companion state (read by the HTTP server for /api/status).
type State struct {
	Connected     bool
	DeviceURL     string
	CPU           string
	RAM           string
	CPUTemp       string
	ActiveTitle   string
	ActiveProcess string
	ActiveProfile string
	Diag          *DeviceDiag
}

// Client manages a persistent WebSocket connection to the CYD Dashboard firmware.
type Client struct {
	cfg              *config.Config
	col              *collector.Collector
	conn             *websocket.Conn
	mu               sync.Mutex
	state            State
	stateMu          sync.RWMutex
	stopCh           chan struct{}
	done             chan struct{}
	onProfileChanged func(string) // called on every profile_changed WS message; set before Run()
}

// SetProfileChangedHandler registers a callback invoked whenever the device
// broadcasts a profile_changed message.  Must be called before Run().
func (c *Client) SetProfileChangedHandler(cb func(string)) {
	c.onProfileChanged = cb
}

func NewClient(cfg *config.Config, col *collector.Collector) *Client {
	return &Client{
		cfg:    cfg,
		col:    col,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Run connects and re-connects indefinitely until Stop is called.
func (c *Client) Run() {
	defer close(c.done)

	backoff := 3 * time.Second
	const maxBackoff = 30 * time.Second
	const rediscoverAfter = 3
	failCount := 0
	currentURL := discovery.Discover(c.cfg.DeviceURL, c.cfg.MdnsHostHint)
	lastCfgURL := c.cfg.DeviceURL
	lastIPURL := "" // last IP-based URL that successfully connected or was discovered

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		// If the user saved a new device URL, re-discover with it immediately.
		if c.cfg.DeviceURL != lastCfgURL {
			lastCfgURL = c.cfg.DeviceURL
			failCount = 0
			backoff = 3 * time.Second
			currentURL = discovery.Discover(c.cfg.DeviceURL, c.cfg.MdnsHostHint)
		}

		resolvedURL, err := c.connectTo(currentURL)
		if err != nil {
			failCount++
			log.Printf("[WS] connection failed (%d): %v — retry in %v", failCount, err, backoff)
			c.setConnected(false, currentURL)

			if failCount >= rediscoverAfter {
				log.Printf("[WS] %d failures, re-running device discovery...", failCount)
				newURL := discovery.Discover(c.cfg.DeviceURL, c.cfg.MdnsHostHint)
				if newURL != currentURL {
					if isHostnameURL(newURL) && lastIPURL != "" {
						// mDNS couldn't resolve to an IP; keep the last working IP URL
						// rather than regressing to an unresolvable hostname.
						log.Printf("[WS] discovery returned hostname, keeping last IP: %s", lastIPURL)
						currentURL = lastIPURL
					} else {
						log.Printf("[WS] discovery found new URL: %s", newURL)
						currentURL = newURL
					}
				}
				failCount = 0
			}

			select {
			case <-time.After(backoff):
			case <-c.stopCh:
				return
			}
			backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			continue
		}

		// connectTo resolved the hostname to an IP — use it for future reconnects
		// so we don't rely on OS mDNS cache (unreliable on Windows after device reboot).
		if resolvedURL != currentURL {
			log.Printf("[WS] using resolved URL for reconnects: %s", resolvedURL)
			currentURL = resolvedURL
		}
		if !isHostnameURL(currentURL) {
			lastIPURL = currentURL
		}

		backoff = 3 * time.Second
		failCount = 0
		c.readLoop()
		c.setConnected(false, currentURL)
		log.Printf("[WS] disconnected, reconnecting...")
	}
}

// Reconnect closes the current connection so Run() will reconnect immediately.
// Call after changing cfg.DeviceURL so the new URL is picked up.
func (c *Client) Reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Stop signals the client to shut down and waits for it to finish.
func (c *Client) Stop() {
	close(c.stopCh)
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()
	<-c.done
}

// GetState returns a snapshot of the current state (safe for concurrent use).
func (c *Client) GetState() State {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// SetFocusInfo updates active window info in state (called by focus ticker in main).
func (c *Client) SetFocusInfo(title, process string) {
	c.stateMu.Lock()
	c.state.ActiveTitle = title
	c.state.ActiveProcess = process
	c.stateMu.Unlock()
}

// SetActiveProfile updates the active profile in state.
func (c *Client) SetActiveProfile(profileID string) {
	c.stateMu.Lock()
	c.state.ActiveProfile = profileID
	c.stateMu.Unlock()
}

// SwitchProfile sends {"action":"switch_profile","id":"..."} to the device via
// the already-open WebSocket. Avoids opening a second TCP connection — the
// ESP32 has a narrow socket window and extra HTTP POSTs stress its heap.
func (c *Client) SwitchProfile(profileID string) error {
	c.stateMu.RLock()
	connected := c.conn != nil
	c.stateMu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected")
	}

	if err := c.sendJSON(map[string]interface{}{
		"action": "switch_profile",
		"id":     profileID,
	}); err != nil {
		return err
	}
	c.SetActiveProfile(profileID)
	return nil
}

// GetProfiles fetches the profile list from the device via HTTP REST.
func (c *Client) GetProfiles() ([]map[string]string, string, error) {
	c.stateMu.RLock()
	deviceURL := c.state.DeviceURL
	c.stateMu.RUnlock()

	if deviceURL == "" {
		return nil, "", fmt.Errorf("not connected")
	}

	httpURL := wsURLToHTTP(deviceURL) + "/api/profiles"
	resp, err := http.Get(httpURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var result struct {
		List     []map[string]string `json:"list"`
		ActiveID string              `json:"active_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}
	return result.List, result.ActiveID, nil
}

func (c *Client) capabilities() []string {
	caps := []string{"system_info"}
	// Only advertise hid_relay where injection actually works (Windows).
	// On macOS/Linux the injector is a stub — advertising it would make the
	// firmware pick COMPANION as the active HID backend and silently drop all
	// keypresses, overriding a working BLE/USB backend.
	if c.cfg.HidRelayEnabled && hid.Supported() {
		caps = append(caps, "hid_relay")
	}
	return caps
}

func (c *Client) setConnected(connected bool, deviceURL string) {
	c.stateMu.Lock()
	c.state.Connected = connected
	if connected {
		c.state.DeviceURL = deviceURL
	} else {
		c.state.DeviceURL = "" // clear so SwitchProfile/GetProfiles don't use stale URL
	}
	c.stateMu.Unlock()
}

// isHostnameURL returns true when the URL host is a DNS name (not a raw IP address).
// Used to avoid regressing from a working IP URL to an unresolvable hostname.
func isHostnameURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	return net.ParseIP(u.Hostname()) == nil
}

// connectTo dials deviceURL and returns the resolved URL (with IP instead of hostname)
// so the caller can use it for future reconnects without relying on OS mDNS cache.
func (c *Client) connectTo(deviceURL string) (string, error) {
	u, err := url.Parse(deviceURL)
	if err != nil {
		return deviceURL, err
	}

	log.Printf("[WS] connecting to %s...", u.String())

	// NetDialContext timeout covers DNS resolution + TCP connect.
	// Without this, resolving ".local" mDNS hostnames on Windows can hang forever.
	netDialer := &net.Dialer{Timeout: 10 * time.Second}
	var resolvedAddr string
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := netDialer.DialContext(ctx, network, addr)
			if err == nil {
				resolvedAddr = c.RemoteAddr().String() // "ip:port"
			}
			return c, err
		},
	}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return deviceURL, err
	}

	// Build an IP-based URL so reconnects bypass OS mDNS resolution
	resolvedURL := deviceURL
	if resolvedAddr != "" {
		host, port, splitErr := net.SplitHostPort(resolvedAddr)
		if splitErr == nil && host != "" && host != u.Hostname() {
			resolvedURL = fmt.Sprintf("ws://%s/ws", net.JoinHostPort(host, port))
		}
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.setConnected(true, deviceURL)

	log.Printf("[WS] connected to CYD Dashboard at %s", u.Host)

	// Sync active profile synchronously so the rules engine always has a valid
	// restore target before the focus ticker's first Evaluate() call.
	// A goroutine here races with the ticker: if the rule fires before the
	// goroutine finishes, preRuleProfile ends up "" and restore never happens.
	if _, activeID, syncErr := c.GetProfiles(); syncErr == nil && activeID != "" {
		c.SetActiveProfile(activeID)
		log.Printf("[WS] synced active profile: %s", activeID)
	}

	hostname, _ := os.Hostname()
	err = c.sendJSON(map[string]interface{}{
		"action":       "companion_hello",
		"version":      version,
		"os":           runtime.GOOS,
		"hostname":     hostname,
		"capabilities": c.capabilities(),
	})
	return resolvedURL, err
}

func (c *Client) readLoop() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	// Start keepalive ping ticker — sends {"action":"ping"} every 25s
	// so the firmware knows we're alive and responds with "pong",
	// which resets our 60s read deadline.
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.sendJSON(map[string]interface{}{"action": "ping"}); err != nil {
					return
				}
			case <-pingDone:
				return
			case <-c.stopCh:
				return
			}
		}
	}()

	// Poll device diagnostics (heap/wifi/uptime) every 5 s.
	// Response arrives as "diag_ack" handled in handleMessage.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = c.sendJSON(map[string]interface{}{"action": "get", "what": "diag"})
			case <-pingDone:
				return
			case <-c.stopCh:
				return
			}
		}
	}()

	defer close(pingDone)

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[WS] read error: %v", err)
			}
			return
		}

		c.handleMessage(data)
	}
}

func (c *Client) handleMessage(data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	action, _ := msg["action"].(string)

	switch action {
	case "get_system_info":
		target, _ := msg["target"].(string)
		id, _ := msg["id"].(string)
		metricId, _ := msg["metric_id"].(string)

		info := c.col.Collect(metricId)

		// Update cached metrics in state
		c.stateMu.Lock()
		switch metricId {
		case "cpu_usage", "cpu":
			c.state.CPU = info
		case "ram_usage", "ram", "memory":
			c.state.RAM = info
		case "cpu_temp", "temperature":
			c.state.CPUTemp = info
		}
		c.stateMu.Unlock()

		log.Printf("[Collector] %s → %s", metricId, info)
		c.sendJSON(map[string]interface{}{
			"action": "return_system_info",
			"target": target,
			"id":     id,
			"info":   info,
		})

	case "get_url":
		target, _ := msg["target"].(string)
		id, _ := msg["id"].(string)
		urlStr, _ := msg["url"].(string)

		go c.fetchURL(target, id, urlStr, msg["headers"], msg["parse"])

	case "profile_changed":
		// Device broadcasts this whenever the active profile changes — from
		// touch UI, companion REST, or web UI.  Always update state so
		// getProfile() is accurate, then notify the rules engine so it can
		// update its restore target when the change was user-initiated.
		if id, ok := msg["id"].(string); ok && id != "" {
			c.SetActiveProfile(id)
			log.Printf("[WS] profile_changed from device → %s", id)
			if c.onProfileChanged != nil {
				c.onProfileChanged(id)
			}
		}

	case "exec_hid":
		var hidMsg hid.ExecHidMsg
		if err := json.Unmarshal(data, &hidMsg); err != nil {
			log.Printf("[HID] bad exec_hid message: %v", err)
			break
		}
		if err := hid.Inject(hidMsg); err != nil {
			log.Printf("[HID] inject error: %v", err)
		}

	case "diag_ack":
		var diag DeviceDiag
		if err := json.Unmarshal(data, &diag); err == nil {
			c.stateMu.Lock()
			c.state.Diag = &diag
			c.stateMu.Unlock()
		}

	case "pong":
		// keepalive, ignore

	default:
		if action != "" && action != "set_macros" && action != "set_widgets" && action != "settings_update" {
			log.Printf("[WS] received: %s", action)
		}
	}
}

// fetchURL performs an HTTP GET on behalf of the firmware and sends return_url_response.
func (c *Client) fetchURL(target, id, urlStr string, headersRaw interface{}, parseRaw interface{}) {
	if urlStr == "" {
		return
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		c.sendJSON(map[string]interface{}{
			"action": "return_url_response", "target": target, "id": id,
			"code": 0, "response": "", "fetch_error": err.Error(),
		})
		return
	}

	// Apply optional headers: [{key:"...", value:"..."}]
	if headers, ok := headersRaw.([]interface{}); ok {
		for _, h := range headers {
			if hm, ok := h.(map[string]interface{}); ok {
				k, _ := hm["key"].(string)
				v, _ := hm["value"].(string)
				if k != "" {
					req.Header.Set(k, v)
				}
			}
		}
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[URL] GET %s error: %v", urlStr, err)
		c.sendJSON(map[string]interface{}{
			"action": "return_url_response", "target": target, "id": id,
			"code": 0, "response": "", "fetch_error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var buf strings.Builder
	io.Copy(&buf, resp.Body)
	body := buf.String()

	response := body
	proxyApplied := false
	if parseRaw != nil && resp.StatusCode == http.StatusOK {
		if pm, ok := parseRaw.(map[string]interface{}); ok {
			parseType, _ := pm["type"].(string)
			tmpl, _ := pm["template"].(string)
			switch parseType {
			case "regex":
				if reStr, _ := pm["regex"].(string); reStr != "" && tmpl != "" {
					if re, reErr := regexp.Compile(reStr); reErr == nil {
						if m := re.FindStringSubmatch(body); m != nil {
							result := tmpl
							for i := 1; i < len(m); i++ {
								result = strings.ReplaceAll(result, fmt.Sprintf("{%d}", i), m[i])
							}
							response = result
							proxyApplied = true
						}
					}
				}
			case "json":
				if keys, ok := pm["json_keys"].([]interface{}); ok && len(keys) > 0 && tmpl != "" {
					var obj interface{}
					if jsonErr := json.Unmarshal([]byte(body), &obj); jsonErr == nil {
						result := tmpl
						for _, k := range keys {
							key, _ := k.(string)
							if key == "" {
								continue
							}
							val := fmt.Sprintf("%v", wsJSONGetPath(obj, key))
							result = strings.ReplaceAll(result, fmt.Sprintf("{%s}", key), val)
						}
						response = result
						proxyApplied = true
					}
				}
			}
		}
	}

	log.Printf("[URL] GET %s → %d (%d bytes, proxy_applied: %v)", urlStr, resp.StatusCode, len(body), proxyApplied)
	reply := map[string]interface{}{
		"action": "return_url_response", "target": target, "id": id,
		"code": resp.StatusCode, "response": response,
	}
	if proxyApplied {
		reply["proxy_applied"] = true
	}
	c.sendJSON(reply)
}

// wsJSONGetPath traverses a dot-notation path through an unmarshalled JSON value.
func wsJSONGetPath(obj interface{}, path string) interface{} {
	cur := obj
	for _, part := range strings.Split(path, ".") {
		if cur == nil {
			return nil
		}
		switch v := cur.(type) {
		case map[string]interface{}:
			cur = v[part]
		case []interface{}:
			idx := 0
			fmt.Sscanf(part, "%d", &idx)
			if idx >= 0 && idx < len(v) {
				cur = v[idx]
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return cur
}

func (c *Client) sendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return nil
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("[WS] write error: %v", err)
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
		return err
	}
	return nil
}

// wsURLToHTTP converts ws://host/ws → http://host
func wsURLToHTTP(wsURL string) string {
	s := wsURL
	if len(s) > 5 && s[:5] == "ws://" {
		s = "http://" + s[5:]
	}
	// Strip /ws path suffix
	if len(s) > 3 && s[len(s)-3:] == "/ws" {
		s = s[:len(s)-3]
	}
	return s
}
