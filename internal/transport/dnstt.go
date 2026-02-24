package transport

import (
	"fmt"

	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
)

func init() {
	Register(&DNSTTProvider{})
}

// DNSTTProvider implements the DNSTT transport (socks backend only).
type DNSTTProvider struct{}

// Type returns the transport type.
func (p *DNSTTProvider) Type() config.TransportType {
	return config.TransportDNSTT
}

// DisplayName returns a human-readable name.
func (p *DNSTTProvider) DisplayName() string {
	return "DNSTT"
}

// SupportedBackends returns the backend types this transport supports.
func (p *DNSTTProvider) SupportedBackends() []config.BackendType {
	return []config.BackendType{config.BackendSOCKS}
}

// RequiredBinaries returns the binaries required for this transport.
func (p *DNSTTProvider) RequiredBinaries(_ config.BackendType) []string {
	return []string{binaries.NameDNSTT}
}

// ValidateConfig validates the tunnel configuration.
func (p *DNSTTProvider) ValidateConfig(tc *config.TunnelConfig) error {
	if tc.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if tc.DNSTT == nil || tc.DNSTT.Pubkey == "" {
		return fmt.Errorf("pubkey is required for dnstt")
	}
	if len(tc.DNSTT.Pubkey) != 64 {
		return fmt.Errorf("pubkey must be 64 hex characters (32 bytes)")
	}
	return nil
}

// BuildArgs builds command line arguments for dnstt-client.
func (p *DNSTTProvider) BuildArgs(tc *config.TunnelConfig, listenPort int, resolver string) (string, []string, error) {
	if err := p.ValidateConfig(tc); err != nil {
		return "", nil, err
	}

	args := []string{
		"-udp", resolver,
		"-pubkey", tc.DNSTT.Pubkey,
		tc.Domain,
		fmt.Sprintf("127.0.0.1:%d", listenPort),
	}

	binary, err := resolveBinary(binaries.NameDNSTT)
	if err != nil {
		return "", nil, err
	}
	return binary, args, nil
}
