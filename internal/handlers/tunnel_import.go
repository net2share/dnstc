package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/clientcfg"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/port"
)

func init() {
	actions.SetHandler(actions.ActionTunnelImport, HandleTunnelImport)
}

// HandleTunnelImport imports a tunnel from a dnstm:// URL.
func HandleTunnelImport(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		cfg = config.Default()
		ctx.Config = cfg
	}

	url := ctx.GetString("url")
	// Also accept URL as first positional argument
	if url == "" && ctx.HasArg(0) {
		url = ctx.GetArg(0)
	}
	if url == "" {
		return fmt.Errorf("URL is required")
	}

	cc, err := clientcfg.Decode(url)
	if err != nil {
		return fmt.Errorf("failed to decode URL: %w", err)
	}

	// Map transport type
	transportType := config.TransportType(cc.Transport.Type)
	if transportType != config.TransportSlipstream && transportType != config.TransportDNSTT {
		return fmt.Errorf("unsupported transport type: %s", cc.Transport.Type)
	}

	// Map backend type
	backendType := config.BackendType(cc.Backend.Type)
	if backendType != config.BackendSOCKS && backendType != config.BackendSSH && backendType != config.BackendShadowsocks {
		return fmt.Errorf("unsupported backend type: %s", cc.Backend.Type)
	}

	// Generate unique tag
	tag := cc.Tag
	if tag == "" {
		tag = config.GenerateUniqueTag(cfg.Tunnels)
	} else {
		tag = config.NormalizeTag(tag)
		if cfg.GetTunnelByTag(tag) != nil {
			tag = config.GenerateUniqueTag(cfg.Tunnels)
		}
	}

	// Auto-assign port
	localPort, err := port.GetAvailable()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	tc := config.TunnelConfig{
		Tag:       tag,
		Transport: transportType,
		Backend:   backendType,
		Domain:    cc.Transport.Domain,
		Port:      localPort,
	}

	configDir := config.ConfigDir()

	// Transport-specific config
	switch transportType {
	case config.TransportSlipstream:
		if cc.Transport.Cert != "" {
			certPath := filepath.Join(configDir, tag+".cert.pem")
			if err := os.WriteFile(certPath, []byte(cc.Transport.Cert), 0644); err != nil {
				return fmt.Errorf("failed to save certificate: %w", err)
			}
			tc.Slipstream = &config.SlipstreamConfig{Cert: certPath}
		}
	case config.TransportDNSTT:
		if cc.Transport.PubKey == "" {
			return fmt.Errorf("DNSTT transport requires a public key")
		}
		tc.DNSTT = &config.DNSTTConfig{Pubkey: cc.Transport.PubKey}
	}

	// Backend-specific config
	switch backendType {
	case config.BackendSSH:
		if cc.Backend.User == "" {
			return fmt.Errorf("SSH backend requires a user")
		}
		sshCfg := &config.SSHConfig{
			User:     cc.Backend.User,
			Password: cc.Backend.Password,
		}
		if cc.Backend.Key != "" {
			keyPath := filepath.Join(configDir, tag+".key.pem")
			if err := os.WriteFile(keyPath, []byte(cc.Backend.Key), 0600); err != nil {
				return fmt.Errorf("failed to save SSH key: %w", err)
			}
			sshCfg.Key = keyPath
		}
		tc.SSH = sshCfg
	case config.BackendShadowsocks:
		method := cc.Backend.Method
		if method == "" {
			method = "aes-256-gcm"
		}
		tc.Shadowsocks = &config.ShadowsocksConfig{
			Server:   "127.0.0.1:8388",
			Password: cc.Backend.Password,
			Method:   method,
		}
	}

	// Validate
	cfg.Tunnels = append(cfg.Tunnels, tc)
	if err := cfg.Validate(); err != nil {
		// Remove the just-added tunnel on validation failure
		cfg.Tunnels = cfg.Tunnels[:len(cfg.Tunnels)-1]
		return fmt.Errorf("validation failed: %w", err)
	}

	// Set as active if no active tunnel
	if cfg.Route.Active == "" {
		cfg.Route.Active = tag
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' imported!", tag))
	ctx.Output.Status(fmt.Sprintf("Transport: %s", config.GetTransportTypeDisplayName(transportType)))
	ctx.Output.Status(fmt.Sprintf("Backend: %s", config.GetBackendTypeDisplayName(backendType)))
	ctx.Output.Status(fmt.Sprintf("Domain: %s", cc.Transport.Domain))
	ctx.Output.Status(fmt.Sprintf("Local port: %d", localPort))

	if cfg.Route.Active == tag {
		ctx.Output.Info("Set as active tunnel")
	}

	return nil
}
