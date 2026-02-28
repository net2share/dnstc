package actions

import (
	"fmt"

	"github.com/net2share/dnstc/internal/config"
)

// TransportOptions returns the available transport options.
func TransportOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "Slipstream",
			Value:       string(config.TransportSlipstream),
			Description: "High-performance DNS tunnel with TLS",
		},
		{
			Label:       "DNSTT",
			Value:       string(config.TransportDNSTT),
			Description: "Classic DNS tunnel (dnstt-client)",
		},
	}
}

// BackendOptionsForTransport returns backend options based on transport type in context.
func BackendOptionsForTransport(ctx *Context) []SelectOption {
	transport := config.TransportType(ctx.GetString("transport"))

	switch transport {
	case config.TransportSlipstream:
		return []SelectOption{
			{
				Label:       "Shadowsocks (SIP003)",
				Value:       string(config.BackendShadowsocks),
				Description: "Slipstream with Shadowsocks plugin",
				Recommended: true,
			},
			{
				Label:       "SOCKS (standalone)",
				Value:       string(config.BackendSOCKS),
				Description: "Slipstream standalone SOCKS proxy",
			},
			{
				Label:       "SSH",
				Value:       string(config.BackendSSH),
				Description: "SSH dynamic forwarding over Slipstream",
			},
		}
	case config.TransportDNSTT:
		return []SelectOption{
			{
				Label:       "SOCKS (standalone)",
				Value:       string(config.BackendSOCKS),
				Description: "DNSTT with SOCKS proxy",
			},
			{
				Label:       "SSH",
				Value:       string(config.BackendSSH),
				Description: "SSH dynamic forwarding over DNSTT",
			},
		}
	default:
		return nil
	}
}

// EncryptionMethodOptions returns the available Shadowsocks encryption methods.
func EncryptionMethodOptions() []SelectOption {
	return []SelectOption{
		{
			Label:       "ChaCha20-IETF-Poly1305",
			Value:       "chacha20-ietf-poly1305",
			Description: "Recommended for most devices",
			Recommended: true,
		},
		{
			Label:       "AES-256-GCM",
			Value:       "aes-256-gcm",
			Description: "Hardware-accelerated on modern CPUs",
		},
		{
			Label:       "AES-128-GCM",
			Value:       "aes-128-gcm",
			Description: "Lighter encryption",
		},
	}
}

// ValidatePubkey validates a DNSTT public key.
func ValidatePubkey(value string) error {
	if len(value) != 64 {
		return NewActionError("public key must be 64 hex characters", "")
	}
	return nil
}

// TunnelPicker provides interactive tunnel selection.
func TunnelPicker(ctx *Context) (string, error) {
	cfg := ctx.Config
	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return "", err
		}
	}

	if len(cfg.Tunnels) == 0 {
		return "", NoTunnelsError()
	}

	var options []SelectOption
	for _, t := range cfg.Tunnels {
		transportName := config.GetTransportTypeDisplayName(t.Transport)
		backendName := config.GetBackendTypeDisplayName(t.Backend)
		label := fmt.Sprintf("%s (%s/%s → %s)", t.Tag, transportName, backendName, t.Domain)
		if t.Tag == cfg.Route.Active {
			label += " [active]"
		}
		options = append(options, SelectOption{
			Label: label,
			Value: t.Tag,
		})
	}

	ctx.Set("_picker_options", options)
	return "", nil
}

// RunningTunnelPicker provides interactive selection of running tunnels.
func RunningTunnelPicker(ctx *Context) (string, error) {
	cfg := ctx.Config
	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return "", err
		}
	}

	if len(cfg.Tunnels) == 0 {
		return "", NoTunnelsError()
	}

	var options []SelectOption
	for _, t := range cfg.Tunnels {
		transportName := config.GetTransportTypeDisplayName(t.Transport)
		backendName := config.GetBackendTypeDisplayName(t.Backend)
		label := fmt.Sprintf("%s (%s/%s → %s)", t.Tag, transportName, backendName, t.Domain)
		options = append(options, SelectOption{
			Label: label,
			Value: t.Tag,
		})
	}

	if len(options) == 0 {
		return "", NewActionError("no running tunnels", "Connect first with 'dnstc up' or use the TUI")
	}

	ctx.Set("_picker_options", options)
	return "", nil
}
