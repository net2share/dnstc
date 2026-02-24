package transport

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
)

func init() {
	Register(&SlipstreamProvider{})
}

// SlipstreamProvider implements the Slipstream transport (both socks and shadowsocks backends).
type SlipstreamProvider struct{}

// Type returns the transport type.
func (p *SlipstreamProvider) Type() config.TransportType {
	return config.TransportSlipstream
}

// DisplayName returns a human-readable name.
func (p *SlipstreamProvider) DisplayName() string {
	return "Slipstream"
}

// SupportedBackends returns the backend types this transport supports.
func (p *SlipstreamProvider) SupportedBackends() []config.BackendType {
	return []config.BackendType{config.BackendSOCKS, config.BackendShadowsocks}
}

// RequiredBinaries returns the binaries required for this transport.
func (p *SlipstreamProvider) RequiredBinaries(backend config.BackendType) []string {
	switch backend {
	case config.BackendShadowsocks:
		return []string{binaries.NameShadowsocks, binaries.NameSlipstream}
	default:
		return []string{binaries.NameSlipstream}
	}
}

// ValidateConfig validates the tunnel configuration.
func (p *SlipstreamProvider) ValidateConfig(tc *config.TunnelConfig) error {
	if tc.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	if tc.Backend == config.BackendShadowsocks {
		if tc.Shadowsocks == nil {
			return fmt.Errorf("shadowsocks config is required for shadowsocks backend")
		}
		if tc.Shadowsocks.Server == "" {
			return fmt.Errorf("shadowsocks server is required")
		}
		if tc.Shadowsocks.Password == "" {
			return fmt.Errorf("shadowsocks password is required")
		}
		if !strings.Contains(tc.Shadowsocks.Server, ":") {
			return fmt.Errorf("invalid shadowsocks server format, expected host:port")
		}
	}

	return nil
}

// BuildArgs builds command line arguments for slipstream.
func (p *SlipstreamProvider) BuildArgs(tc *config.TunnelConfig, listenPort int, resolver string) (string, []string, error) {
	if err := p.ValidateConfig(tc); err != nil {
		return "", nil, err
	}

	switch tc.Backend {
	case config.BackendShadowsocks:
		return p.buildSIP003Args(tc, listenPort, resolver)
	default:
		return p.buildSOCKSArgs(tc, listenPort, resolver)
	}
}

// buildSOCKSArgs builds args for slipstream-client standalone SOCKS mode.
func (p *SlipstreamProvider) buildSOCKSArgs(tc *config.TunnelConfig, listenPort int, resolver string) (string, []string, error) {
	args := []string{
		"--domain", tc.Domain,
		"--resolver", resolver,
		"--tcp-listen-port", fmt.Sprintf("%d", listenPort),
	}

	if tc.Slipstream != nil && tc.Slipstream.Cert != "" {
		args = append(args, "--cert", tc.Slipstream.Cert)
	}

	binary, err := resolveBinary(binaries.NameSlipstream)
	if err != nil {
		return "", nil, err
	}
	return binary, args, nil
}

// buildSIP003Args builds args for sslocal with slipstream as SIP003 plugin.
func (p *SlipstreamProvider) buildSIP003Args(tc *config.TunnelConfig, listenPort int, resolver string) (string, []string, error) {
	method := tc.Shadowsocks.Method
	if method == "" {
		method = "aes-256-gcm"
	}

	pluginPath, err := resolveBinary(binaries.NameSlipstream)
	if err != nil {
		return "", nil, err
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%d", listenPort)
	pluginOpts := fmt.Sprintf("domain=%s;resolver=%s;", tc.Domain, resolver)

	args := []string{
		"-s", tc.Shadowsocks.Server,
		"-k", tc.Shadowsocks.Password,
		"-m", method,
		"-b", listenAddr,
		"--plugin", pluginPath,
		"--plugin-opts", pluginOpts,
	}

	binary, err := resolveBinary(binaries.NameShadowsocks)
	if err != nil {
		return "", nil, err
	}
	return binary, args, nil
}
