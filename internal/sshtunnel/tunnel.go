// Package sshtunnel provides an SSH tunnel with local SOCKS5 dynamic forwarding.
// It connects to a local TCP port (provided by a DNS transport) and creates an
// SSH connection, then exposes a local SOCKS5 proxy that forwards traffic through
// the SSH tunnel.
package sshtunnel

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config configures an SSH tunnel.
type Config struct {
	TransportAddr string // local address of the DNS transport (e.g., "127.0.0.1:12345")
	SOCKSAddr     string // local SOCKS5 listen address (e.g., "127.0.0.1:1080")
	User          string
	Password      string
	KeyPath       string // path to PEM private key file
}

// Tunnel manages an SSH connection and local SOCKS5 proxy.
type Tunnel struct {
	cfg      Config
	client   *ssh.Client
	listener net.Listener
	wg       sync.WaitGroup
	done     chan struct{}
}

// Start establishes the SSH connection and starts the SOCKS5 listener.
func Start(cfg Config) (*Tunnel, error) {
	// Build SSH auth methods
	var auths []ssh.AuthMethod
	if cfg.KeyPath != "" {
		keyData, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read SSH key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("parse SSH key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
		auths = append(auths, ssh.KeyboardInteractive(
			func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = cfg.Password
				}
				return answers, nil
			},
		))
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("no SSH auth method configured")
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Connect to the DNS transport's local port with retries.
	// DNS tunnels may need a moment after the port is open before
	// the session is fully established and can relay SSH traffic.
	var client *ssh.Client
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second)
		}
		tcpConn, err := net.DialTimeout("tcp", cfg.TransportAddr, 10*time.Second)
		if err != nil {
			lastErr = fmt.Errorf("dial transport: %w", err)
			continue
		}
		sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, cfg.TransportAddr, sshCfg)
		if err != nil {
			tcpConn.Close()
			lastErr = fmt.Errorf("SSH handshake: %w", err)
			continue
		}
		client = ssh.NewClient(sshConn, chans, reqs)
		break
	}
	if client == nil {
		return nil, lastErr
	}

	// Start local SOCKS5 listener
	listener, err := net.Listen("tcp", cfg.SOCKSAddr)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("listen SOCKS: %w", err)
	}

	t := &Tunnel{
		cfg:      cfg,
		client:   client,
		listener: listener,
		done:     make(chan struct{}),
	}

	t.wg.Add(1)
	go t.acceptLoop()

	return t, nil
}

// Addr returns the SOCKS5 listener address.
func (t *Tunnel) Addr() string {
	return t.listener.Addr().String()
}

// Stop shuts down the tunnel.
func (t *Tunnel) Stop() {
	close(t.done)
	t.listener.Close()
	t.client.Close()
	t.wg.Wait()
}

// IsAlive returns true if the SSH connection is still responding.
func (t *Tunnel) IsAlive() bool {
	_, _, err := t.client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

func (t *Tunnel) acceptLoop() {
	defer t.wg.Done()
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				continue
			}
		}
		t.wg.Add(1)
		go t.handleConn(conn)
	}
}

func (t *Tunnel) handleConn(conn net.Conn) {
	defer t.wg.Done()
	defer conn.Close()

	target, err := socks5Handshake(conn)
	if err != nil {
		return
	}

	// Dial through SSH
	remote, err := t.client.Dial("tcp", target)
	if err != nil {
		socks5Reply(conn, 0x05) // connection refused
		return
	}
	defer remote.Close()

	// Success reply
	socks5Reply(conn, 0x00)

	// Bidirectional relay
	var relayWg sync.WaitGroup
	relayWg.Add(2)
	go func() {
		defer relayWg.Done()
		io.Copy(remote, conn)
	}()
	go func() {
		defer relayWg.Done()
		io.Copy(conn, remote)
	}()
	relayWg.Wait()
}
