package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/net2share/dnstc/internal/engine"
)

// Server listens on a Unix socket and dispatches IPC requests to the engine.
type Server struct {
	socketPath string
	eng        *engine.Engine
	version    string
	listener   net.Listener
	wg         sync.WaitGroup
	ShutdownCh chan struct{}
}

// NewServer creates a new IPC server.
func NewServer(socketPath, version string, eng *engine.Engine) *Server {
	return &Server{
		socketPath: socketPath,
		eng:        eng,
		version:    version,
		ShutdownCh: make(chan struct{}, 1),
	}
}

// Start removes any stale socket and begins accepting connections.
func (s *Server) Start() error {
	// Remove stale socket
	if _, err := os.Stat(s.socketPath); err == nil {
		os.Remove(s.socketPath)
	}

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.socketPath, err)
	}

	// Restrict socket permissions
	os.Chmod(s.socketPath, 0600)

	s.listener = ln

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

// Stop closes the listener, waits for in-flight requests, and removes the socket.
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	os.Remove(s.socketPath)
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer conn.Close()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	// Allow large messages (e.g. config payload)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			encoder.Encode(Response{Error: "invalid request"})
			continue
		}

		resp := s.dispatch(&req)
		encoder.Encode(resp)
	}
}

func (s *Server) dispatch(req *Request) Response {
	switch req.Method {
	case MethodPing:
		return s.resultJSON(PingResult{Version: s.version, PID: os.Getpid()})

	case MethodShutdown:
		select {
		case s.ShutdownCh <- struct{}{}:
		default:
		}
		return s.ok()

	case MethodStart:
		if err := s.eng.Start(); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodStop:
		if err := s.eng.Stop(); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodStartTunnel:
		tag, err := s.parseTag(req)
		if err != nil {
			return s.errResp(err)
		}
		if err := s.eng.StartTunnel(tag); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodStopTunnel:
		tag, err := s.parseTag(req)
		if err != nil {
			return s.errResp(err)
		}
		if err := s.eng.StopTunnel(tag); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodRestartTunnel:
		tag, err := s.parseTag(req)
		if err != nil {
			return s.errResp(err)
		}
		if err := s.eng.RestartTunnel(tag); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodActivateTunnel:
		tag, err := s.parseTag(req)
		if err != nil {
			return s.errResp(err)
		}
		if err := s.eng.ActivateTunnel(tag); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodStatus:
		status := s.eng.Status()
		return s.resultJSON(status)

	case MethodGetConfig:
		cfg := s.eng.GetConfig()
		return s.resultJSON(cfg)

	case MethodReloadConfig:
		if err := s.eng.ReloadConfig(); err != nil {
			return s.errResp(err)
		}
		return s.ok()

	case MethodIsConnected:
		return s.resultJSON(BoolResult{Value: s.eng.IsConnected()})

	default:
		return Response{Error: fmt.Sprintf("unknown method: %s", req.Method)}
	}
}

func (s *Server) parseTag(req *Request) (string, error) {
	if req.Params == nil {
		return "", fmt.Errorf("missing params")
	}
	var p TagParam
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.Tag == "" {
		return "", fmt.Errorf("tag is required")
	}
	return p.Tag, nil
}

func (s *Server) ok() Response {
	return Response{}
}

func (s *Server) errResp(err error) Response {
	return Response{Error: err.Error()}
}

func (s *Server) resultJSON(v any) Response {
	data, err := json.Marshal(v)
	if err != nil {
		return s.errResp(err)
	}
	return Response{Result: data}
}
