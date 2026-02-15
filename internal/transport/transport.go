// Package transport provides transport provider abstraction for dnstc.
package transport

import (
	"github.com/net2share/dnstc/internal/config"
)

// Transport defines the interface that all transport providers must implement.
type Transport interface {
	// Type returns the transport type identifier.
	Type() config.TransportType

	// DisplayName returns a human-readable name for display.
	DisplayName() string

	// RequiredBinaries returns the list of binaries required by this transport.
	// The backend type determines which additional binaries are needed.
	RequiredBinaries(backend config.BackendType) []string

	// SupportedBackends returns the backend types this transport supports.
	SupportedBackends() []config.BackendType

	// ValidateConfig validates the tunnel configuration.
	ValidateConfig(tc *config.TunnelConfig) error

	// BuildArgs builds the command line arguments for the transport.
	// Returns the binary path and arguments.
	BuildArgs(tc *config.TunnelConfig, listenPort int, resolver string) (binary string, args []string, err error)
}
