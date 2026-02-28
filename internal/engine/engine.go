// Package engine provides the core runtime for dnstc.
// It manages tunnel processes (as child processes) and the TCP gateway.
package engine

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/dnsproxy"
	"github.com/net2share/dnstc/internal/gateway"
	"github.com/net2share/dnstc/internal/port"
	"github.com/net2share/dnstc/internal/process"
	"github.com/net2share/dnstc/internal/sshtunnel"
	"github.com/net2share/dnstc/internal/transport"
)

// singleton engine instance
var (
	instance EngineController
	mu       sync.RWMutex
)

// Set sets the global engine instance.
func Set(e EngineController) {
	mu.Lock()
	defer mu.Unlock()
	instance = e
}

// Get returns the global engine instance, or nil if not running.
func Get() EngineController {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

// Status represents the current state of all tunnels and the gateway.
type Status struct {
	Active       string                   `json:"active"`
	GatewayAddr  string                   `json:"gateway_addr"`
	DNSProxyAddr string                   `json:"dns_proxy_addr"`
	Tunnels      map[string]*TunnelStatus `json:"tunnels"`
}

// TunnelStatus represents the status of a single tunnel.
type TunnelStatus struct {
	Tag       string               `json:"tag"`
	Transport config.TransportType `json:"transport"`
	Backend   config.BackendType   `json:"backend"`
	Domain    string               `json:"domain"`
	Running   bool                 `json:"running"`
	Active    bool                 `json:"active"`
	Port      int                  `json:"port"`
}

// Engine manages the full dnstc runtime: tunnel processes and gateway.
type Engine struct {
	cfg        *config.Config
	procMgr    *process.Manager
	gw         *gateway.Gateway
	dnsProxy   *dnsproxy.Proxy
	sshTunnels map[string]*sshtunnel.Tunnel
	mu         sync.RWMutex
}

// New creates a new engine with the given configuration.
func New(cfg *config.Config) *Engine {
	return &Engine{
		cfg:        cfg,
		procMgr:    process.NewManager(config.StatePath()),
		sshTunnels: make(map[string]*sshtunnel.Tunnel),
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

	// Stop SSH tunnels first (they depend on transport processes)
	for tag, st := range e.sshTunnels {
		st.Stop()
		delete(e.sshTunnels, tag)
	}

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

	// Stop SSH tunnel first (depends on transport process)
	if st, ok := e.sshTunnels[tag]; ok {
		st.Stop()
		delete(e.sshTunnels, tag)
	}

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

	// Stop SSH tunnel if running
	if st, ok := e.sshTunnels[tag]; ok {
		st.Stop()
		delete(e.sshTunnels, tag)
	}

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

		// For SSH tunnels, also check the SSH tunnel itself
		if tc.Backend == config.BackendSSH {
			if st, ok := e.sshTunnels[tc.Tag]; ok {
				ts.Running = ts.Running && st.IsAlive()
			} else {
				ts.Running = false
			}
		}

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

	// Check required binaries are installed
	mgr := binaries.NewManager()
	defs := binaries.Defs()
	for _, name := range t.RequiredBinaries(tc.Backend) {
		def := defs[name]
		if !mgr.IsInstalled(def) {
			return fmt.Errorf("binary %s not installed — run 'dnstc install' first", name)
		}
	}

	// For SSH backend, we need two ports:
	//   internalPort: DNS transport process listens here (raw TCP → SSH)
	//   exposedPort:  SSH SOCKS5 proxy listens here (what gateway routes to)
	// For other backends, transport process listens on the exposed port directly.
	isSSH := tc.Backend == config.BackendSSH

	exposedPort := tc.Port
	if exposedPort == 0 {
		exposedPort = extractPort(e.cfg.Listen.SOCKS)
		if exposedPort == 0 {
			exposedPort = 1080
		}
	}

	transportPort := exposedPort
	if isSSH {
		// Auto-assign an internal port for the transport process
		internalPort, err := port.GetAvailable()
		if err != nil {
			return fmt.Errorf("failed to find internal port for SSH tunnel: %w", err)
		}
		transportPort = internalPort
	} else {
		if !port.IsAvailable(transportPort) {
			return fmt.Errorf("port %d is already in use", transportPort)
		}
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

	// Build args — transport process always listens on transportPort
	binary, args, err := t.BuildArgs(tc, transportPort, resolver)
	if err != nil {
		return fmt.Errorf("failed to build args: %w", err)
	}

	// Start transport process
	if err := e.procMgr.Start(processName, binary, args); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	// For SSH backend, start SSH tunnel asynchronously.
	// The transport needs time to establish the DNS session before SSH can connect.
	if isSSH {
		transportAddr := fmt.Sprintf("127.0.0.1:%d", transportPort)
		socksAddr := fmt.Sprintf("127.0.0.1:%d", exposedPort)

		sshCfg := sshtunnel.Config{
			TransportAddr: transportAddr,
			SOCKSAddr:     socksAddr,
			User:          tc.SSH.User,
			Password:      tc.SSH.Password,
			KeyPath:       tc.SSH.Key,
		}

		go func() {
			if err := waitForPort(transportAddr, 10*time.Second); err != nil {
				fmt.Printf("warning: transport for %q did not become ready: %v\n", tag, err)
				e.procMgr.Stop(processName)
				return
			}

			st, err := sshtunnel.Start(sshCfg)
			if err != nil {
				fmt.Printf("warning: SSH tunnel %q failed: %v\n", tag, err)
				e.procMgr.Stop(processName)
				return
			}

			e.mu.Lock()
			e.sshTunnels[tag] = st
			e.mu.Unlock()
		}()
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

	// For SSH backend, verify the SSH tunnel is alive
	if tc.Backend == config.BackendSSH {
		st, ok := e.sshTunnels[activeTag]
		if !ok || !st.IsAlive() {
			return ""
		}
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
	// Also check SSH tunnels (they run in-process, not as child processes)
	return len(e.sshTunnels) > 0
}

// waitForPort polls a TCP address until it accepts connections or the timeout expires.
func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
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
