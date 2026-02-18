// Package engine provides the core runtime for dnstc.
// It manages tunnel processes (as child processes) and the TCP gateway.
package engine

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/dnsproxy"
	"github.com/net2share/dnstc/internal/download"
	"github.com/net2share/dnstc/internal/gateway"
	"github.com/net2share/dnstc/internal/port"
	"github.com/net2share/dnstc/internal/process"
	"github.com/net2share/dnstc/internal/transport"
)

// singleton engine instance
var (
	instance *Engine
	mu       sync.RWMutex
)

// Set sets the global engine instance.
func Set(e *Engine) {
	mu.Lock()
	defer mu.Unlock()
	instance = e
}

// Get returns the global engine instance, or nil if not running.
func Get() *Engine {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

// Status represents the current state of all tunnels and the gateway.
type Status struct {
	Active       string
	GatewayAddr  string
	DNSProxyAddr string
	Tunnels      map[string]*TunnelStatus
}

// TunnelStatus represents the status of a single tunnel.
type TunnelStatus struct {
	Tag       string
	Transport config.TransportType
	Backend   config.BackendType
	Domain    string
	Running   bool
	Active    bool
	Port      int
}

// Engine manages the full dnstc runtime: tunnel processes and gateway.
type Engine struct {
	cfg      *config.Config
	procMgr  *process.Manager
	gw       *gateway.Gateway
	dnsProxy *dnsproxy.Proxy
	mu       sync.RWMutex
}

// New creates a new engine with the given configuration.
func New(cfg *config.Config) *Engine {
	return &Engine{
		cfg:     cfg,
		procMgr: process.NewManager(config.StatePath()),
	}
}

// Start starts all enabled tunnels and the gateway.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Start DNS proxy first (before tunnels need it)
	if err := e.startDNSProxyLocked(); err != nil {
		// Non-fatal: fall back to direct resolver
		fmt.Printf("warning: dns proxy failed to start: %v (using direct resolvers)\n", err)
	}

	// Start gateway
	if err := e.startGatewayLocked(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	// Start all enabled tunnels
	for _, tc := range e.cfg.Tunnels {
		if !tc.IsEnabled() {
			continue
		}
		if err := e.startTunnelLocked(tc.Tag); err != nil {
			// Log but don't fail — start as many as possible
			fmt.Printf("warning: failed to start tunnel %q: %v\n", tc.Tag, err)
		}
	}

	return nil
}

// Stop stops all tunnels and the gateway.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Stop all tunnel processes
	e.procMgr.StopAll()

	// Stop gateway
	if e.gw != nil {
		e.gw.Stop()
		e.gw = nil
	}

	// Stop DNS proxy last (tunnels may still need it during shutdown)
	if e.dnsProxy != nil {
		e.dnsProxy.Stop(context.Background())
		e.dnsProxy = nil
	}

	return nil
}

// StartTunnel starts a specific tunnel by tag.
func (e *Engine) StartTunnel(tag string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Ensure DNS proxy is running (non-fatal)
	if e.dnsProxy == nil {
		if err := e.startDNSProxyLocked(); err != nil {
			fmt.Printf("warning: dns proxy failed to start: %v (using direct resolvers)\n", err)
		}
	}

	if err := e.startTunnelLocked(tag); err != nil {
		return err
	}

	// Ensure gateway is running
	if e.gw == nil {
		if err := e.startGatewayLocked(); err != nil {
			return fmt.Errorf("tunnel started but gateway failed: %w", err)
		}
	}

	return nil
}

// StopTunnel stops a specific tunnel by tag.
func (e *Engine) StopTunnel(tag string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	processName := "tunnel-" + tag
	if err := e.procMgr.Stop(processName); err != nil {
		return err
	}

	// If no tunnels are running, stop the gateway
	if !e.hasRunningTunnelsLocked() && e.gw != nil {
		e.gw.Stop()
		e.gw = nil
	}

	return nil
}

// RestartTunnel restarts a specific tunnel by tag.
func (e *Engine) RestartTunnel(tag string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	processName := "tunnel-" + tag
	e.procMgr.Stop(processName)

	return e.startTunnelLocked(tag)
}

// ActivateTunnel sets a tunnel as the active route and saves config.
func (e *Engine) ActivateTunnel(tag string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tc := e.cfg.GetTunnelByTag(tag)
	if tc == nil {
		return fmt.Errorf("tunnel %q not found", tag)
	}

	e.cfg.Route.Active = tag
	return e.cfg.Save()
}

// Status returns the current status of all tunnels and the gateway.
func (e *Engine) Status() *Status {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s := &Status{
		Active:  e.cfg.Route.Active,
		Tunnels: make(map[string]*TunnelStatus),
	}

	if e.gw != nil {
		s.GatewayAddr = e.gw.Addr()
	}

	if e.dnsProxy != nil && e.dnsProxy.IsRunning() {
		s.DNSProxyAddr = e.dnsProxy.Addr()
	}

	for _, tc := range e.cfg.Tunnels {
		ts := &TunnelStatus{
			Tag:       tc.Tag,
			Transport: tc.Transport,
			Backend:   tc.Backend,
			Domain:    tc.Domain,
			Active:    tc.Tag == e.cfg.Route.Active,
			Port:      tc.Port,
		}

		processName := "tunnel-" + tc.Tag
		ts.Running = e.procMgr.IsRunning(processName)

		s.Tunnels[tc.Tag] = ts
	}

	return s
}

// GetConfig returns the current configuration.
func (e *Engine) GetConfig() *config.Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg
}

// ReloadConfig reloads configuration from disk.
func (e *Engine) ReloadConfig() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	e.cfg = cfg
	return nil
}

func (e *Engine) startTunnelLocked(tag string) error {
	tc := e.cfg.GetTunnelByTag(tag)
	if tc == nil {
		return fmt.Errorf("tunnel %q not found", tag)
	}

	processName := "tunnel-" + tag
	if e.procMgr.IsRunning(processName) {
		return fmt.Errorf("tunnel %q is already running", tag)
	}

	// Get transport provider
	t, err := transport.Get(tc.Transport)
	if err != nil {
		return fmt.Errorf("failed to get transport provider: %w", err)
	}

	// Ensure required binaries
	for _, binary := range t.RequiredBinaries(tc.Backend) {
		if !download.IsBinaryInstalled(binary) {
			if err := download.EnsureBinary(binary, nil); err != nil {
				return fmt.Errorf("failed to download %s: %w", binary, err)
			}
		}
	}

	// Determine listen port
	listenPort := tc.Port
	if listenPort == 0 {
		listenPort = extractPort(e.cfg.Listen.SOCKS)
		if listenPort == 0 {
			listenPort = 1080
		}
	}

	if !port.IsAvailable(listenPort) {
		return fmt.Errorf("port %d is already in use", listenPort)
	}

	// Determine resolver: per-tunnel override > DNS proxy > global fallback
	var resolver string
	if tc.Resolver != "" {
		resolver = tc.Resolver
	} else if e.dnsProxy != nil && e.dnsProxy.IsRunning() {
		resolver = e.dnsProxy.Addr()
	} else {
		resolver = e.cfg.GetResolver(tc)
	}

	// Build args
	binary, args, err := t.BuildArgs(tc, listenPort, resolver)
	if err != nil {
		return fmt.Errorf("failed to build args: %w", err)
	}

	// Start process
	if err := e.procMgr.Start(processName, binary, args); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	return nil
}

func (e *Engine) startDNSProxyLocked() error {
	if len(e.cfg.Resolvers) == 0 {
		return nil // nothing to proxy
	}

	p := dnsproxy.New(e.cfg.Resolvers)
	if err := p.Start(context.Background()); err != nil {
		return err
	}

	e.dnsProxy = p
	return nil
}

func (e *Engine) startGatewayLocked() error {
	if e.gw != nil {
		return nil // already running
	}

	gwAddr := e.cfg.Listen.SOCKS
	if gwAddr == "" {
		gwAddr = "127.0.0.1:1080"
	}

	// If configured port is taken, auto-assign an available one
	gwPort := extractPort(gwAddr)
	if gwPort > 0 && !port.IsAvailable(gwPort) {
		newPort, err := port.GetAvailable()
		if err != nil {
			return fmt.Errorf("gateway port %d in use and no available port found: %w", gwPort, err)
		}
		gwAddr = fmt.Sprintf("127.0.0.1:%d", newPort)
		// Update config so status reflects the actual port
		e.cfg.Listen.SOCKS = gwAddr
		e.cfg.Save()
	}

	e.gw = gateway.New(gwAddr, e.resolveActiveTarget)
	return e.gw.Start()
}

// resolveActiveTarget returns the address of the active tunnel for the gateway.
// Called per-connection so activate takes effect immediately.
func (e *Engine) resolveActiveTarget() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	activeTag := e.cfg.Route.Active
	if activeTag == "" {
		return ""
	}

	tc := e.cfg.GetTunnelByTag(activeTag)
	if tc == nil {
		return ""
	}

	tunnelPort := tc.Port
	if tunnelPort == 0 {
		tunnelPort = extractPort(e.cfg.Listen.SOCKS)
	}
	if tunnelPort == 0 {
		return ""
	}

	// Check if the tunnel is actually running
	processName := "tunnel-" + activeTag
	if !e.procMgr.IsRunning(processName) {
		return ""
	}

	return fmt.Sprintf("127.0.0.1:%d", tunnelPort)
}

// IsConnected returns true if any tunnels are currently running.
func (e *Engine) IsConnected() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.hasRunningTunnelsLocked()
}

func (e *Engine) hasRunningTunnelsLocked() bool {
	for _, tc := range e.cfg.Tunnels {
		processName := "tunnel-" + tc.Tag
		if e.procMgr.IsRunning(processName) {
			return true
		}
	}
	return false
}

func extractPort(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return p
}
