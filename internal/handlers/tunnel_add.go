package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/port"
)

func init() {
	actions.SetHandler(actions.ActionTunnelAdd, HandleTunnelAdd)
}

// HandleTunnelAdd adds a new tunnel.
func HandleTunnelAdd(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		cfg = config.Default()
		// Ensure default gateway port is available
		gwPort := 1080
		if !port.IsAvailable(gwPort) {
			if p, pErr := port.GetAvailable(); pErr == nil {
				cfg.Listen.SOCKS = fmt.Sprintf("127.0.0.1:%d", p)
			}
		}
		ctx.Config = cfg
	}

	transportStr := ctx.GetString("transport")
	backendStr := ctx.GetString("backend")
	domain := ctx.GetString("domain")
	tag := ctx.GetString("tag")

	if transportStr == "" || backendStr == "" || domain == "" {
		return fmt.Errorf("--transport, --backend, and --domain flags are required")
	}

	transportType := config.TransportType(transportStr)
	backendType := config.BackendType(backendStr)

	// Validate transport
	if transportType != config.TransportSlipstream && transportType != config.TransportDNSTT {
		return fmt.Errorf("invalid transport type: %s (must be slipstream or dnstt)", transportType)
	}

	// Validate backend compatibility
	if transportType == config.TransportDNSTT && backendType == config.BackendShadowsocks {
		return actions.NewActionError("incompatible transport and backend", "DNSTT does not support Shadowsocks backend")
	}

	// Generate tag if not provided
	if tag == "" {
		tag = config.GenerateUniqueTag(cfg.Tunnels)
	}

	tag = config.NormalizeTag(tag)
	if err := config.ValidateTag(tag); err != nil {
		return fmt.Errorf("invalid tag: %w", err)
	}

	if cfg.GetTunnelByTag(tag) != nil {
		return actions.TunnelExistsError(tag)
	}

	// Determine local port
	localPort := ctx.GetInt("port")
	if localPort == 0 {
		p, err := port.GetAvailable()
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		localPort = p
	}

	// Build tunnel config
	tc := config.TunnelConfig{
		Tag:       tag,
		Transport: transportType,
		Backend:   backendType,
		Domain:    domain,
		Port:      localPort,
	}

	// Transport-specific config
	switch transportType {
	case config.TransportSlipstream:
		if backendType == config.BackendShadowsocks {
			ssServer := ctx.GetString("ss-server")
			ssPassword := ctx.GetString("ss-password")
			ssMethod := ctx.GetString("ss-method")
			if ssServer == "" || ssPassword == "" {
				return fmt.Errorf("--ss-server and --ss-password are required for Shadowsocks backend")
			}
			if ssMethod == "" {
				ssMethod = "chacha20-ietf-poly1305"
			}
			tc.Shadowsocks = &config.ShadowsocksConfig{
				Server:   ssServer,
				Password: ssPassword,
				Method:   ssMethod,
			}
		} else {
			cert := ctx.GetString("cert")
			if cert != "" {
				tc.Slipstream = &config.SlipstreamConfig{Cert: cert}
			}
		}
	case config.TransportDNSTT:
		pubkey := ctx.GetString("pubkey")
		if pubkey == "" {
			return fmt.Errorf("--pubkey is required for DNSTT transport")
		}
		if len(pubkey) != 64 {
			return fmt.Errorf("public key must be 64 hex characters")
		}
		tc.DNSTT = &config.DNSTTConfig{Pubkey: pubkey}
	}

	// Add to config
	cfg.Tunnels = append(cfg.Tunnels, tc)
	if cfg.Route.Active == "" {
		cfg.Route.Active = tag
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' created!", tag))
	ctx.Output.Status(fmt.Sprintf("Transport: %s", config.GetTransportTypeDisplayName(transportType)))
	ctx.Output.Status(fmt.Sprintf("Backend: %s", config.GetBackendTypeDisplayName(backendType)))
	ctx.Output.Status(fmt.Sprintf("Domain: %s", domain))
	ctx.Output.Status(fmt.Sprintf("Local port: %d", localPort))

	if cfg.Route.Active == tag {
		ctx.Output.Info("Set as active tunnel")
	}

	return nil
}
