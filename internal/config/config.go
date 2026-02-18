// Package config provides configuration management for dnstc.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultResolver is the fallback DNS resolver used when none is configured.
const DefaultResolver = "1.1.1.1:53"

// Config holds the dnstc configuration.
type Config struct {
	Log       LogConfig      `json:"log,omitempty"`
	Listen    ListenConfig   `json:"listen,omitempty"`
	Resolvers []string       `json:"resolvers,omitempty"`
	Tunnels   []TunnelConfig `json:"tunnels,omitempty"`
	Route     RouteConfig    `json:"route,omitempty"`
}

// LogConfig configures logging behavior.
type LogConfig struct {
	Level string `json:"level,omitempty"`
}

// ListenConfig holds local listener configuration.
type ListenConfig struct {
	SOCKS string `json:"socks,omitempty"`
}

// RouteConfig configures routing and active tunnel.
type RouteConfig struct {
	Active string `json:"active,omitempty"`
}

// Default returns a default configuration.
func Default() *Config {
	return &Config{
		Log: LogConfig{
			Level: "info",
		},
		Listen: ListenConfig{
			SOCKS: "127.0.0.1:1080",
		},
		Resolvers: []string{DefaultResolver},
		Tunnels:   []TunnelConfig{},
		Route:     RouteConfig{},
	}
}

// Load reads the configuration from the default path.
func Load() (*Config, error) {
	return LoadFromPath(Path())
}

// LoadFromPath reads the configuration from a specific path.
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// LoadOrDefault reads the configuration from disk, or returns a default config if not found.
func LoadOrDefault() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		// Config file not found from our own error
		if _, statErr := os.Stat(Path()); os.IsNotExist(statErr) {
			return Default(), nil
		}
		return nil, err
	}
	return cfg, nil
}

// Save writes the configuration to the default path.
func (c *Config) Save() error {
	return c.SaveToPath(Path())
}

// SaveToPath writes the configuration to a specific path.
func (c *Config) SaveToPath(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0640); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetTunnelByTag returns a tunnel by its tag.
func (c *Config) GetTunnelByTag(tag string) *TunnelConfig {
	for i := range c.Tunnels {
		if c.Tunnels[i].Tag == tag {
			return &c.Tunnels[i]
		}
	}
	return nil
}

// GetResolver returns the resolver to use for a tunnel.
func (c *Config) GetResolver(tc *TunnelConfig) string {
	// Tunnel-specific resolver takes precedence
	if tc.Resolver != "" {
		return tc.Resolver
	}

	// Fall back to global resolvers
	if len(c.Resolvers) > 0 {
		return c.Resolvers[0]
	}

	return DefaultResolver
}

// GetFormattedConfig returns the configuration as a formatted JSON string.
func (c *Config) GetFormattedConfig() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}
