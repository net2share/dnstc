// Package dnsproxy provides an embedded local DNS proxy with health-aware
// parallel upstream resolution.
package dnsproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/net2share/dnstc/internal/port"
)

const (
	cacheSizeBytes = 4 * 1024 * 1024 // 4 MB
	cacheMinTTL    = 30              // seconds
	cacheMaxTTL    = 300             // seconds
)

// Proxy wraps a dnsproxy server with health-aware upstream management.
type Proxy struct {
	upstreamAddrs []string
	proxy         *proxy.Proxy
	upstream      *HealthAwareUpstream
	listenPort    int
	mu            sync.RWMutex
	running       bool
}

// New creates a new DNS proxy for the given upstream addresses.
func New(upstreams []string) *Proxy {
	return &Proxy{
		upstreamAddrs: upstreams,
	}
}

// Start initializes upstreams, starts the DNS server, and begins health monitoring.
func (p *Proxy) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	// Create upstream instances with silent logger
	silentLogger := slog.New(slog.DiscardHandler)
	opts := &upstream.Options{
		Logger: silentLogger,
	}
	var ups []upstream.Upstream
	for _, addr := range p.upstreamAddrs {
		u, err := upstream.AddressToUpstream(addr, opts)
		if err != nil {
			// Close already-created upstreams
			for _, created := range ups {
				created.Close()
			}
			return fmt.Errorf("failed to create upstream %q: %w", addr, err)
		}
		ups = append(ups, u)
	}

	// Create health-aware upstream
	p.upstream = NewHealthAwareUpstream(ups)

	// Find a port available on both TCP and UDP
	listenPort, err := port.GetAvailableDual()
	if err != nil {
		p.upstream.Close()
		p.upstream = nil
		return fmt.Errorf("failed to find available port: %w", err)
	}
	p.listenPort = listenPort

	// Configure dnsproxy
	listenIP := net.ParseIP("127.0.0.1")
	cfg := &proxy.Config{
		UDPListenAddr: []*net.UDPAddr{
			{IP: listenIP, Port: listenPort},
		},
		TCPListenAddr: []*net.TCPAddr{
			{IP: listenIP, Port: listenPort},
		},
		UpstreamConfig: &proxy.UpstreamConfig{
			Upstreams: []upstream.Upstream{p.upstream},
		},
		UpstreamMode:   proxy.UpstreamModeParallel,
		CacheEnabled:   true,
		CacheSizeBytes: cacheSizeBytes,
		CacheMinTTL:    cacheMinTTL,
		CacheMaxTTL:    cacheMaxTTL,
		// Silence dnsproxy's own logging
		Logger: slog.New(slog.DiscardHandler),
	}

	dnsProxy, err := proxy.New(cfg)
	if err != nil {
		p.upstream.Close()
		p.upstream = nil
		return fmt.Errorf("failed to create dns proxy: %w", err)
	}

	if err := dnsProxy.Start(ctx); err != nil {
		p.upstream.Close()
		p.upstream = nil
		return fmt.Errorf("failed to start dns proxy: %w", err)
	}

	p.proxy = dnsProxy
	p.running = true

	return nil
}

// Stop shuts down the DNS proxy and health monitor.
func (p *Proxy) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	var firstErr error

	if p.proxy != nil {
		if err := p.proxy.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		p.proxy = nil
	}

	if p.upstream != nil {
		if err := p.upstream.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		p.upstream = nil
	}

	p.running = false
	return firstErr
}

// Addr returns the proxy's listen address (e.g., "127.0.0.1:54321").
func (p *Proxy) Addr() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return ""
	}
	return fmt.Sprintf("127.0.0.1:%d", p.listenPort)
}

// IsRunning reports whether the proxy is currently running.
func (p *Proxy) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// UpstreamStatuses returns health status of all configured upstreams.
func (p *Proxy) UpstreamStatuses() []UpstreamStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.upstream == nil {
		return nil
	}
	return p.upstream.GetStatus()
}
