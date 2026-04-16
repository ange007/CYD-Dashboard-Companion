package config

import (
	"encoding/json"
	"os"
)

// FocusRule maps a regex pattern for active window/process name to a CYD profile ID.
type FocusRule struct {
	Match     string `json:"match"`
	ProfileID string `json:"profile_id"`
}

type Config struct {
	DeviceURL         string      `json:"device_url"`
	MdnsHostHint      string      `json:"mdns_host_hint"`      // hostname fragment to match during mDNS scan, default "cyd"
	PushIntervalMs    int         `json:"push_interval_ms"`
	LocalPort         int         `json:"local_port"`
	CommandsAllowlist []string    `json:"commands_allowlist"`
	FocusRules        []FocusRule `json:"focus_rules"`
	HidRelayEnabled   bool        `json:"hid_relay_enabled"`
}

func Default() *Config {
	return &Config{
		DeviceURL:         "",
		MdnsHostHint:      "cyd",
		PushIntervalMs:    2000,
		LocalPort:         9800,
		CommandsAllowlist: []string{},
		FocusRules:        []FocusRule{},
		HidRelayEnabled:   true,
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
