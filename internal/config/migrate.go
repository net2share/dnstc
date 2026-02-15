package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// OldConfig represents the old single-transport YAML configuration format.
type OldConfig struct {
	Transport   OldTransportConfig `yaml:"transport"`
	Resolvers   []string           `yaml:"resolvers,omitempty"`
	Shadowsocks OldShadowsocks     `yaml:"shadowsocks,omitempty"`
	Listen      OldListenConfig    `yaml:"listen"`
	Mode        string             `yaml:"mode"`
}

// OldMultiConfig represents the multi-transport YAML configuration format.
type OldMultiConfig struct {
	Listen     OldListenConfig                `yaml:"listen"`
	Active     string                         `yaml:"active,omitempty"`
	Resolvers  []string                       `yaml:"resolvers,omitempty"`
	Transports map[string]*OldTransportDetail `yaml:"transports,omitempty"`
}

// OldTransportConfig represents old transport configuration.
type OldTransportConfig struct {
	Type     string `yaml:"type"`
	Domain   string `yaml:"domain"`
	CertPath string `yaml:"cert_path,omitempty"`
	Pubkey   string `yaml:"pubkey,omitempty"`
}

// OldTransportDetail represents a transport in the multi-transport format.
type OldTransportDetail struct {
	Type        string          `yaml:"type"`
	Domain      string          `yaml:"domain"`
	CertPath    string          `yaml:"cert_path,omitempty"`
	Pubkey      string          `yaml:"pubkey,omitempty"`
	Resolver    string          `yaml:"resolver,omitempty"`
	Shadowsocks *OldShadowsocks `yaml:"shadowsocks,omitempty"`
}

// OldShadowsocks represents old Shadowsocks configuration.
type OldShadowsocks struct {
	Server   string `yaml:"server,omitempty"`
	Password string `yaml:"password,omitempty"`
	Method   string `yaml:"method,omitempty"`
}

// OldListenConfig represents old listen configuration.
type OldListenConfig struct {
	SOCKS string `yaml:"socks"`
	HTTP  string `yaml:"http,omitempty"`
}

// MigrateConfigIfNeeded checks for old YAML config and migrates to JSON.
func MigrateConfigIfNeeded() error {
	jsonPath := Path()
	yamlPath := OldConfigPath()

	// If JSON config exists, no migration needed
	if _, err := os.Stat(jsonPath); err == nil {
		return nil
	}

	// Check if YAML config exists
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config to migrate
		}
		return err
	}

	// Detect which YAML format we have
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return err
	}

	var newCfg *Config

	_, hasTransports := rawConfig["transports"]
	_, hasTransport := rawConfig["transport"]
	_, hasMode := rawConfig["mode"]

	if hasTransports {
		// Multi-transport YAML format
		newCfg, err = migrateMultiTransport(data)
	} else if hasTransport || hasMode {
		// Old single-transport format
		newCfg, err = migrateSingleTransport(data)
	} else {
		return nil // Unknown format
	}

	if err != nil {
		return err
	}
	if newCfg == nil {
		return nil
	}

	// Backup old config
	backupPath := yamlPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0640); err != nil {
		return err
	}

	// Save new JSON config
	return newCfg.Save()
}

// migrateSingleTransport migrates the original single-transport YAML format.
func migrateSingleTransport(data []byte) (*Config, error) {
	var oldCfg OldConfig
	if err := yaml.Unmarshal(data, &oldCfg); err != nil {
		return nil, err
	}

	if oldCfg.Transport.Domain == "" {
		return nil, nil
	}

	newCfg := Default()
	if oldCfg.Listen.SOCKS != "" {
		newCfg.Listen.SOCKS = oldCfg.Listen.SOCKS
	}
	if len(oldCfg.Resolvers) > 0 {
		newCfg.Resolvers = oldCfg.Resolvers
	}

	tc := convertOldTransport(oldCfg.Transport.Type, oldCfg.Mode, &OldTransportDetail{
		Type:     oldCfg.Transport.Type,
		Domain:   oldCfg.Transport.Domain,
		CertPath: oldCfg.Transport.CertPath,
		Pubkey:   oldCfg.Transport.Pubkey,
		Shadowsocks: &OldShadowsocks{
			Server:   oldCfg.Shadowsocks.Server,
			Password: oldCfg.Shadowsocks.Password,
			Method:   oldCfg.Shadowsocks.Method,
		},
	})

	tag := GenerateName()
	tc.Tag = tag
	newCfg.Tunnels = append(newCfg.Tunnels, *tc)
	newCfg.Route.Active = tag

	return newCfg, nil
}

// migrateMultiTransport migrates the multi-transport YAML format.
func migrateMultiTransport(data []byte) (*Config, error) {
	var oldCfg OldMultiConfig
	if err := yaml.Unmarshal(data, &oldCfg); err != nil {
		return nil, err
	}

	newCfg := Default()
	if oldCfg.Listen.SOCKS != "" {
		newCfg.Listen.SOCKS = oldCfg.Listen.SOCKS
	}
	if len(oldCfg.Resolvers) > 0 {
		newCfg.Resolvers = oldCfg.Resolvers
	}

	activeTag := ""
	for name, td := range oldCfg.Transports {
		tc := convertOldTransport(td.Type, "", td)

		// Use existing name as tag, normalizing it
		tag := NormalizeTag(name)
		if err := ValidateTag(tag); err != nil {
			tag = GenerateName()
		}
		tc.Tag = tag

		if td.Resolver != "" {
			tc.Resolver = td.Resolver
		}

		newCfg.Tunnels = append(newCfg.Tunnels, *tc)

		if name == oldCfg.Active {
			activeTag = tag
		}
	}

	if activeTag != "" {
		newCfg.Route.Active = activeTag
	}

	return newCfg, nil
}

// convertOldTransport maps old transport types to new transport+backend.
func convertOldTransport(transportType, mode string, td *OldTransportDetail) *TunnelConfig {
	tc := &TunnelConfig{
		Domain: td.Domain,
	}

	switch {
	case transportType == "slipstream-sip003" || mode == "sip003":
		tc.Transport = TransportSlipstream
		tc.Backend = BackendShadowsocks
		if td.CertPath != "" {
			tc.Slipstream = &SlipstreamConfig{Cert: td.CertPath}
		}
		if td.Shadowsocks != nil && td.Shadowsocks.Server != "" {
			method := td.Shadowsocks.Method
			if method == "" {
				method = "aes-256-gcm"
			}
			tc.Shadowsocks = &ShadowsocksConfig{
				Server:   td.Shadowsocks.Server,
				Password: td.Shadowsocks.Password,
				Method:   method,
			}
		}

	case transportType == "slipstream-socks":
		tc.Transport = TransportSlipstream
		tc.Backend = BackendSOCKS
		if td.CertPath != "" {
			tc.Slipstream = &SlipstreamConfig{Cert: td.CertPath}
		}

	case transportType == "dnstt-socks" || transportType == "dnstt":
		tc.Transport = TransportDNSTT
		tc.Backend = BackendSOCKS
		if td.Pubkey != "" {
			tc.DNSTT = &DNSTTConfig{Pubkey: td.Pubkey}
		}

	default:
		// Default to slipstream + socks
		tc.Transport = TransportSlipstream
		tc.Backend = BackendSOCKS
		if td.CertPath != "" {
			tc.Slipstream = &SlipstreamConfig{Cert: td.CertPath}
		}
	}

	return tc
}

// LoadOrMigrate loads config, migrating from old YAML format if necessary.
func LoadOrMigrate() (*Config, error) {
	if err := MigrateConfigIfNeeded(); err != nil {
		log.Printf("config migration warning: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		// If JSON still doesn't exist, return default
		if _, statErr := os.Stat(Path()); os.IsNotExist(statErr) {
			return Default(), nil
		}
		return nil, err
	}
	return cfg, nil
}
