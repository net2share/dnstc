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

// GetAvailableDual finds a port available on both TCP and UDP (needed for DNS).
func GetAvailableDual() (int, error) {
	// Let OS assign a TCP port, then verify UDP is also free
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}

	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Verify UDP is also available on that port
	udpAddr := fmt.Sprintf("127.0.0.1:%d", p)
	pc, err := net.ListenPacket("udp", udpAddr)
	if err != nil {
		// Rare: TCP free but UDP taken; fall back to range scan
		return getAvailableDualInRange(MinPort, MaxPort)
	}
	pc.Close()

	return p, nil
}

func getAvailableDualInRange(min, max int) (int, error) {
	for p := min; p <= max; p++ {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			ln.Close()
			continue
		}
		ln.Close()
		pc.Close()
		return p, nil
	}
	return 0, fmt.Errorf("no dual-stack port found in range %d-%d", min, max)
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
