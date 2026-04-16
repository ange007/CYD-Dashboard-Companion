package collector

import (
	"fmt"
	"strings"
	"time"

	gocpu "github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	gnet "github.com/shirou/gopsutil/v3/net"
)

// buildMetrics collects all metrics and returns the full catalog.
func buildMetrics(c *Collector) []Metric {
	var out []Metric

	// ── CPU ──────────────────────────────────────────────────────────────────
	if pct, err := gocpu.Percent(200*time.Millisecond, false); err == nil && len(pct) > 0 {
		out = append(out, Metric{"cpu.usage", "CPU Usage", "CPU", "%", fmtFloat(pct[0], 1)})
	}
	if n, err := gocpu.Counts(true); err == nil {
		out = append(out, Metric{"cpu.cores_logical", "CPU Cores (Logical)", "CPU", "", fmt.Sprintf("%d", n)})
	}
	if n, err := gocpu.Counts(false); err == nil {
		out = append(out, Metric{"cpu.cores_physical", "CPU Cores (Physical)", "CPU", "", fmt.Sprintf("%d", n)})
	}
	if info, err := gocpu.Info(); err == nil && len(info) > 0 {
		out = append(out, Metric{"cpu.freq", "CPU Frequency", "CPU", "MHz", fmtFloat(info[0].Mhz, 0)})
	}

	// ── Memory ───────────────────────────────────────────────────────────────
	if v, err := mem.VirtualMemory(); err == nil {
		out = append(out, Metric{"mem.used_pct", "RAM Usage", "Memory", "%", fmtFloat(v.UsedPercent, 1)})
		out = append(out, Metric{"mem.used_gb", "RAM Used", "Memory", "GB", fmtGB(v.Used)})
		out = append(out, Metric{"mem.total_gb", "RAM Total", "Memory", "GB", fmtGB(v.Total)})
		out = append(out, Metric{"mem.available_gb", "RAM Available", "Memory", "GB", fmtGB(v.Available)})
	}
	if s, err := mem.SwapMemory(); err == nil && s.Total > 0 {
		out = append(out, Metric{"swap.used_pct", "Swap Usage", "Memory", "%", fmtFloat(s.UsedPercent, 1)})
		out = append(out, Metric{"swap.used_gb", "Swap Used", "Memory", "GB", fmtGB(s.Used)})
	}

	// ── Disk ─────────────────────────────────────────────────────────────────
	if parts, err := disk.Partitions(false); err == nil {
		for _, p := range parts {
			usage, err := disk.Usage(p.Mountpoint)
			if err != nil || usage.Total == 0 {
				continue
			}
			// Sanitise mount point for use in metric ID (replace \ / : with -)
			slug := strings.NewReplacer("\\", "-", "/", "-", ":", "").Replace(p.Mountpoint)
			slug = strings.Trim(slug, "-")
			if slug == "" {
				slug = "root"
			}
			label := p.Mountpoint
			out = append(out, Metric{
				"disk." + slug + ".used_pct",
				"Disk " + label + " Usage", "Disk", "%",
				fmtFloat(usage.UsedPercent, 1),
			})
			out = append(out, Metric{
				"disk." + slug + ".free_gb",
				"Disk " + label + " Free", "Disk", "GB",
				fmtGB(usage.Free),
			})
		}
	}

	// ── Network ──────────────────────────────────────────────────────────────
	if counters, err := gnet.IOCounters(false); err == nil && len(counters) > 0 {
		now := time.Now()
		sent := counters[0].BytesSent
		recv := counters[0].BytesRecv

		c.mu.Lock()
		if !c.lastNetTime.IsZero() {
			dt := now.Sub(c.lastNetTime).Seconds()
			if dt > 0 {
				sentPS := float64(sent-c.lastNetSent) / dt / 1024 / 1024 // MB/s
				recvPS := float64(recv-c.lastNetRecv) / dt / 1024 / 1024
				if sentPS < 0 { sentPS = 0 }
				if recvPS < 0 { recvPS = 0 }
				out = append(out, Metric{"net.sent_mbps", "Network Sent", "Network", "MB/s", fmtFloat(sentPS, 2)})
				out = append(out, Metric{"net.recv_mbps", "Network Recv", "Network", "MB/s", fmtFloat(recvPS, 2)})
			}
		}
		c.lastNetTime = now
		c.lastNetSent = sent
		c.lastNetRecv = recv
		c.mu.Unlock()
	}

	// ── Host ─────────────────────────────────────────────────────────────────
	if uptime, err := host.Uptime(); err == nil {
		out = append(out, Metric{"host.uptime_h", "Uptime", "System", "h", fmtFloat(float64(uptime)/3600, 1)})
	}

	// Temperatures (dynamic — sensor names vary per system)
	if temps, err := host.SensorsTemperatures(); err == nil {
		seen := map[string]bool{}
		for _, t := range temps {
			if t.Temperature <= 0 {
				continue
			}
			slug := strings.NewReplacer(" ", "_", ".", "_", "/", "_").Replace(strings.ToLower(t.SensorKey))
			id := "host.temp." + slug
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, Metric{
				id,
				"Temp: " + t.SensorKey, "Sensors", "°C",
				fmtFloat(t.Temperature, 1),
			})
		}
	}

	return out
}
