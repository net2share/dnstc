package transport

import (
	"fmt"
	"sync"

	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
)

var (
	registry = make(map[config.TransportType]Transport)
	mu       sync.RWMutex
)

// Register adds a transport to the registry.
func Register(t Transport) {
	mu.Lock()
	defer mu.Unlock()
	registry[t.Type()] = t
}

// Get returns a transport by type.
func Get(tt config.TransportType) (Transport, error) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := registry[tt]
	if !ok {
		return nil, fmt.Errorf("unknown transport: %s", tt)
	}
	return t, nil
}

// GetAll returns all registered transports.
func GetAll() []Transport {
	mu.RLock()
	defer mu.RUnlock()
	transports := make([]Transport, 0, len(registry))
	for _, t := range registry {
		transports = append(transports, t)
	}
	return transports
}

// Types returns all available transport types.
func Types() []config.TransportType {
	return config.GetTransportTypes()
}

// resolveBinary resolves a binary path via the binaries manager.
func resolveBinary(name string) (string, error) {
	mgr := binaries.NewManager()
	defs := binaries.Defs()
	def, ok := defs[name]
	if !ok {
		return "", fmt.Errorf("unknown binary: %s", name)
	}
	return mgr.ResolvePath(def)
}
