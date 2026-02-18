package config

// ApplyDefaults fills in missing optional values with defaults.
func (c *Config) ApplyDefaults() {
	// Log defaults
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	// Listen defaults
	if c.Listen.SOCKS == "" {
		c.Listen.SOCKS = "127.0.0.1:1080"
	}

	// Resolvers default
	if len(c.Resolvers) == 0 {
		c.Resolvers = []string{DefaultResolver}
	}

	// Tunnel defaults
	for i := range c.Tunnels {
		t := &c.Tunnels[i]

		// Enabled defaults to true
		if t.Enabled == nil {
			enabled := true
			t.Enabled = &enabled
		}

		// Shadowsocks method default
		if t.Backend == BackendShadowsocks && t.Shadowsocks != nil {
			if t.Shadowsocks.Method == "" {
				t.Shadowsocks.Method = "aes-256-gcm"
			}
		}
	}

	// Route active defaults to first enabled tunnel
	if c.Route.Active == "" && len(c.Tunnels) > 0 {
		for _, t := range c.Tunnels {
			if t.IsEnabled() {
				c.Route.Active = t.Tag
				break
			}
		}
	}
}
