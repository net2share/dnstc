// Package port provides port allocation utilities.
package port

import (
	"fmt"
	"net"
)

const (
	// MinPort is the minimum port number for dynamic allocation.
	MinPort = 10000
	// MaxPort is the maximum port number for dynamic allocation.
	MaxPort = 60000
)

// GetPort tries to get the preferred port, or finds an available one.
func GetPort(preferred int) (int, error) {
	if preferred > 0 && IsAvailable(preferred) {
		return preferred, nil
	}

	return GetAvailable()
}

// IsAvailable checks if a port is available for binding.
func IsAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// GetAvailable finds an available port in the dynamic range.
func GetAvailable() (int, error) {
	// Let the OS assign a port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// GetAvailableInRange finds an available port in the specified range.
func GetAvailableInRange(min, max int) (int, error) {
	for port := min; port <= max; port++ {
		if IsAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", min, max)
}
