// Package ipc provides the daemon IPC protocol over Unix sockets.
package ipc

import "encoding/json"

// IPC method constants.
const (
	MethodPing           = "ping"
	MethodShutdown       = "shutdown"
	MethodStart          = "start"
	MethodStop           = "stop"
	MethodStartTunnel    = "start_tunnel"
	MethodStopTunnel     = "stop_tunnel"
	MethodRestartTunnel  = "restart_tunnel"
	MethodActivateTunnel = "activate_tunnel"
	MethodStatus         = "status"
	MethodGetConfig      = "get_config"
	MethodReloadConfig   = "reload_config"
	MethodIsConnected    = "is_connected"
)

// Request is an IPC request sent from client to server.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is an IPC response sent from server to client.
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// TagParam carries a tunnel tag for tunnel-specific methods.
type TagParam struct {
	Tag string `json:"tag"`
}

// PingResult is the response payload for the ping method.
type PingResult struct {
	Version string `json:"version"`
	PID     int    `json:"pid"`
}

// BoolResult wraps a boolean response value.
type BoolResult struct {
	Value bool `json:"value"`
}
