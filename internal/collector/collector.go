package collector

import (
	"fmt"
	"sync"
	"time"
)

// Metric is a single measurable system value exposed to the CYD Dashboard.
type Metric struct {
	ID       string `json:"id"`       // unique key, e.g. "cpu.usage"
	Label    string `json:"label"`    // human-readable, e.g. "CPU Usage"
	Category string `json:"category"` // grouping, e.g. "CPU"
	Unit     string `json:"unit"`     // display unit, e.g. "%"
	Value    string `json:"value"`    // current snapshot value
}

// Collector builds and refreshes the metrics catalog.
type Collector struct {
	mu      sync.RWMutex
	catalog []Metric

	// network delta state
	lastNetTime  time.Time
	lastNetSent  uint64
	lastNetRecv  uint64
}

func New() *Collector {
	c := &Collector{}
	c.refresh() // initial population (static fields + first values)
	return c
}

// Catalog returns a copy of the current metrics catalog with up-to-date values.
func (c *Collector) Catalog() []Metric {
	c.refresh()
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Metric, len(c.catalog))
	copy(out, c.catalog)
	return out
}

// Collect returns the current value for a single metric ID.
// Returns "0" when the ID is unknown or the metric fails.
func (c *Collector) Collect(id string) string {
	c.refresh()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, m := range c.catalog {
		if m.ID == id {
			return m.Value
		}
	}
	return "0"
}

// refresh rebuilds the catalog with fresh values.
func (c *Collector) refresh() {
	metrics := buildMetrics(c)
	c.mu.Lock()
	c.catalog = metrics
	c.mu.Unlock()
}

func fmtFloat(v float64, prec int) string {
	return fmt.Sprintf(fmt.Sprintf("%%.%df", prec), v)
}

func fmtGB(bytes uint64) string {
	return fmtFloat(float64(bytes)/1e9, 2)
}
