package config

import (
	"fmt"
	"regexp"
)

var tagRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if err := c.validateTagUniqueness(); err != nil {
		return err
	}

	if err := c.validateTunnels(); err != nil {
		return err
	}

	if err := c.validateRoute(); err != nil {
		return err
	}

	return nil
}

// validateTagUniqueness ensures all tunnel tags are unique.
func (c *Config) validateTagUniqueness() error {
	tunnelTags := make(map[string]bool)
	for i, t := range c.Tunnels {
		if t.Tag == "" {
			return fmt.Errorf("tunnels[%d]: tag is required", i)
		}
		if !tagRegex.MatchString(t.Tag) {
			return fmt.Errorf("tunnel '%s': tag must start with a lowercase letter and contain only lowercase letters, numbers, and hyphens", t.Tag)
		}
		if tunnelTags[t.Tag] {
			return fmt.Errorf("duplicate tunnel tag: %s", t.Tag)
		}
		tunnelTags[t.Tag] = true
	}
	return nil
}

// validateTunnels validates all tunnel configurations.
func (c *Config) validateTunnels() error {
	for _, t := range c.Tunnels {
		if t.Transport == "" {
			return fmt.Errorf("tunnel '%s': transport is required", t.Tag)
		}

		if t.Transport != TransportSlipstream && t.Transport != TransportDNSTT {
			return fmt.Errorf("tunnel '%s': unknown transport %s", t.Tag, t.Transport)
		}

		if t.Backend == "" {
			return fmt.Errorf("tunnel '%s': backend is required", t.Tag)
		}

		if t.Backend != BackendSOCKS && t.Backend != BackendShadowsocks {
			return fmt.Errorf("tunnel '%s': unknown backend %s", t.Tag, t.Backend)
		}

		if t.Domain == "" {
			return fmt.Errorf("tunnel '%s': domain is required", t.Tag)
		}

		// Check transport-backend compatibility
		if err := validateTransportBackendCompatibility(t.Transport, t.Backend); err != nil {
			return fmt.Errorf("tunnel '%s': %w", t.Tag, err)
		}

		// Transport-specific validation
		switch t.Transport {
		case TransportSlipstream:
			// Cert is optional
		case TransportDNSTT:
			if t.DNSTT == nil || t.DNSTT.Pubkey == "" {
				return fmt.Errorf("tunnel '%s': dnstt.pubkey is required", t.Tag)
			}
			if len(t.DNSTT.Pubkey) != 64 {
				return fmt.Errorf("tunnel '%s': dnstt.pubkey must be 64 hex characters", t.Tag)
			}
		}

		// Backend-specific validation
		if t.Backend == BackendShadowsocks {
			if t.Shadowsocks == nil {
				return fmt.Errorf("tunnel '%s': shadowsocks config is required", t.Tag)
			}
			if t.Shadowsocks.Server == "" {
				return fmt.Errorf("tunnel '%s': shadowsocks.server is required", t.Tag)
			}
			if t.Shadowsocks.Password == "" {
				return fmt.Errorf("tunnel '%s': shadowsocks.password is required", t.Tag)
			}
			if err := validateShadowsocksMethod(t.Shadowsocks.Method); err != nil {
				return fmt.Errorf("tunnel '%s': %w", t.Tag, err)
			}
		}
	}

	return nil
}

// validateRoute validates route configuration.
func (c *Config) validateRoute() error {
	if c.Route.Active != "" {
		if c.GetTunnelByTag(c.Route.Active) == nil {
			return fmt.Errorf("route.active: tunnel '%s' does not exist", c.Route.Active)
		}
	}
	return nil
}

// validateTransportBackendCompatibility checks if a transport and backend are compatible.
func validateTransportBackendCompatibility(transport TransportType, backend BackendType) error {
	if transport == TransportDNSTT && backend == BackendShadowsocks {
		return fmt.Errorf("dnstt transport does not support shadowsocks backend")
	}
	return nil
}

// validateShadowsocksMethod validates the shadowsocks encryption method.
func validateShadowsocksMethod(method string) error {
	if method == "" {
		return nil // Default will be applied
	}
	validMethods := []string{
		"aes-256-gcm",
		"aes-128-gcm",
		"chacha20-ietf-poly1305",
	}
	for _, m := range validMethods {
		if method == m {
			return nil
		}
	}
	return fmt.Errorf("invalid shadowsocks method '%s', must be one of: aes-256-gcm, aes-128-gcm, chacha20-ietf-poly1305", method)
}
