package config

// TransportType defines the type of transport.
type TransportType string

const (
	TransportSlipstream TransportType = "slipstream"
	TransportDNSTT      TransportType = "dnstt"
)

// BackendType defines the type of backend.
type BackendType string

const (
	BackendSOCKS       BackendType = "socks"
	BackendSSH         BackendType = "ssh"
	BackendShadowsocks BackendType = "shadowsocks"
)

// TunnelConfig configures a DNS tunnel.
type TunnelConfig struct {
	Tag         string             `json:"tag"`
	Enabled     *bool              `json:"enabled,omitempty"`
	Transport   TransportType      `json:"transport"`
	Backend     BackendType        `json:"backend"`
	Domain      string             `json:"domain"`
	Port        int                `json:"port,omitempty"`
	Resolver    string             `json:"resolver,omitempty"`
	Slipstream  *SlipstreamConfig  `json:"slipstream,omitempty"`
	DNSTT       *DNSTTConfig       `json:"dnstt,omitempty"`
	Shadowsocks *ShadowsocksConfig `json:"shadowsocks,omitempty"`
	SSH         *SSHConfig         `json:"ssh,omitempty"`
}

// SlipstreamConfig holds Slipstream-specific configuration.
type SlipstreamConfig struct {
	Cert string `json:"cert,omitempty"`
}

// DNSTTConfig holds DNSTT-specific configuration.
type DNSTTConfig struct {
	Pubkey string `json:"pubkey"`
}

// ShadowsocksConfig holds Shadowsocks configuration for SIP003 mode.
type ShadowsocksConfig struct {
	Server   string `json:"server"`
	Password string `json:"password"`
	Method   string `json:"method,omitempty"`
}

// SSHConfig holds SSH backend configuration.
type SSHConfig struct {
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	Key      string `json:"key,omitempty"` // path to PEM private key file
}

// IsEnabled returns true if the tunnel is enabled.
func (t *TunnelConfig) IsEnabled() bool {
	return t.Enabled == nil || *t.Enabled
}

// IsSlipstream returns true if this is a Slipstream tunnel.
func (t *TunnelConfig) IsSlipstream() bool {
	return t.Transport == TransportSlipstream
}

// IsDNSTT returns true if this is a DNSTT tunnel.
func (t *TunnelConfig) IsDNSTT() bool {
	return t.Transport == TransportDNSTT
}

// GetTransportTypeDisplayName returns a human-readable name for a transport type.
func GetTransportTypeDisplayName(t TransportType) string {
	switch t {
	case TransportSlipstream:
		return "Slipstream"
	case TransportDNSTT:
		return "DNSTT"
	default:
		return string(t)
	}
}

// GetBackendTypeDisplayName returns a human-readable name for a backend type.
func GetBackendTypeDisplayName(b BackendType) string {
	switch b {
	case BackendSOCKS:
		return "SOCKS"
	case BackendSSH:
		return "SSH"
	case BackendShadowsocks:
		return "Shadowsocks"
	default:
		return string(b)
	}
}

// GetTransportTypes returns all available transport types.
func GetTransportTypes() []TransportType {
	return []TransportType{
		TransportSlipstream,
		TransportDNSTT,
	}
}

// GetBackendTypes returns all available backend types.
func GetBackendTypes() []BackendType {
	return []BackendType{
		BackendSOCKS,
		BackendSSH,
		BackendShadowsocks,
	}
}
