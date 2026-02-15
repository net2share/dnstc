// Package gateway provides a TCP relay proxy for routing traffic
// through the active tunnel.
package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Gateway is a TCP relay that listens on a local port and forwards
// connections to the active tunnel's port.
type Gateway struct {
	addr     string
	listener net.Listener
	target   func() string // returns "host:port" of active tunnel
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// New creates a new gateway. targetFunc is called per-connection to
// resolve the current active tunnel's address.
func New(addr string, targetFunc func() string) *Gateway {
	ctx, cancel := context.WithCancel(context.Background())
	return &Gateway{
		addr:   addr,
		target: targetFunc,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins accepting connections on the gateway port.
func (g *Gateway) Start() error {
	ln, err := net.Listen("tcp", g.addr)
	if err != nil {
		g.cancel()
		return fmt.Errorf("gateway: failed to listen on %s: %w", g.addr, err)
	}
	g.listener = ln

	g.wg.Add(1)
	go g.acceptLoop()

	return nil
}

// Stop shuts down the gateway and waits for active connections to drain.
func (g *Gateway) Stop() error {
	g.cancel()
	if g.listener != nil {
		g.listener.Close()
	}
	g.wg.Wait()
	return nil
}

// Addr returns the actual listen address (useful when port was auto-assigned).
func (g *Gateway) Addr() string {
	if g.listener != nil {
		return g.listener.Addr().String()
	}
	return g.addr
}

func (g *Gateway) acceptLoop() {
	defer g.wg.Done()

	for {
		conn, err := g.listener.Accept()
		if err != nil {
			select {
			case <-g.ctx.Done():
				return
			default:
				continue
			}
		}

		g.wg.Add(1)
		go g.handleConn(conn)
	}
}

func (g *Gateway) handleConn(src net.Conn) {
	defer g.wg.Done()
	defer src.Close()

	target := g.target()
	if target == "" {
		return
	}

	dst, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		return
	}
	defer dst.Close()

	errc := make(chan error, 2)
	go func() { _, err := io.Copy(dst, src); errc <- err }()
	go func() { _, err := io.Copy(src, dst); errc <- err }()

	// Wait for first direction to finish; deferred Close()s terminate the other.
	<-errc
}
