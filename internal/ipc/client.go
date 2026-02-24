package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
)

// compile-time check
var _ engine.EngineController = (*Client)(nil)

// Client connects to the daemon over a Unix socket and implements EngineController.
type Client struct {
	conn    net.Conn
	scanner *bufio.Scanner
	mu      sync.Mutex
}

// Dial connects to the daemon socket.
func Dial(socketPath string) (*Client, error) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	return &Client{conn: conn, scanner: scanner}, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Ping verifies the daemon is alive.
func (c *Client) Ping() (*PingResult, error) {
	resp, err := c.call(MethodPing, nil)
	if err != nil {
		return nil, err
	}
	var result PingResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("invalid ping response: %w", err)
	}
	return &result, nil
}

// Shutdown asks the daemon to exit.
func (c *Client) Shutdown() error {
	_, err := c.call(MethodShutdown, nil)
	return err
}

func (c *Client) Start() error {
	_, err := c.call(MethodStart, nil)
	return err
}

func (c *Client) Stop() error {
	_, err := c.call(MethodStop, nil)
	return err
}

func (c *Client) StartTunnel(tag string) error {
	_, err := c.call(MethodStartTunnel, TagParam{Tag: tag})
	return err
}

func (c *Client) StopTunnel(tag string) error {
	_, err := c.call(MethodStopTunnel, TagParam{Tag: tag})
	return err
}

func (c *Client) RestartTunnel(tag string) error {
	_, err := c.call(MethodRestartTunnel, TagParam{Tag: tag})
	return err
}

func (c *Client) ActivateTunnel(tag string) error {
	_, err := c.call(MethodActivateTunnel, TagParam{Tag: tag})
	return err
}

func (c *Client) Status() *engine.Status {
	resp, err := c.call(MethodStatus, nil)
	if err != nil {
		return &engine.Status{Tunnels: make(map[string]*engine.TunnelStatus)}
	}
	var s engine.Status
	if err := json.Unmarshal(resp.Result, &s); err != nil {
		return &engine.Status{Tunnels: make(map[string]*engine.TunnelStatus)}
	}
	if s.Tunnels == nil {
		s.Tunnels = make(map[string]*engine.TunnelStatus)
	}
	return &s
}

func (c *Client) GetConfig() *config.Config {
	resp, err := c.call(MethodGetConfig, nil)
	if err != nil {
		return config.Default()
	}
	var cfg config.Config
	if err := json.Unmarshal(resp.Result, &cfg); err != nil {
		return config.Default()
	}
	return &cfg
}

func (c *Client) ReloadConfig() error {
	_, err := c.call(MethodReloadConfig, nil)
	return err
}

func (c *Client) IsConnected() bool {
	resp, err := c.call(MethodIsConnected, nil)
	if err != nil {
		return false
	}
	var result BoolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false
	}
	return result.Value
}

func (c *Client) call(method string, params any) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := Request{Method: method}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		req.Params = data
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write newline-delimited JSON
	data = append(data, '\n')
	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read response
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return nil, fmt.Errorf("connection closed")
	}

	var resp Response
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	return &resp, nil
}

// DetectDaemon checks if a daemon is running and returns a connected client.
func DetectDaemon() (bool, *Client) {
	socketPath := config.SocketPath()

	// Check if socket file exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return false, nil
	}

	// Try to connect
	client, err := Dial(socketPath)
	if err != nil {
		// Stale socket — remove it
		os.Remove(socketPath)
		return false, nil
	}

	// Verify daemon is alive
	if _, err := client.Ping(); err != nil {
		client.Close()
		os.Remove(socketPath)
		return false, nil
	}

	return true, client
}
