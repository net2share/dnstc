// Package process provides process lifecycle management for dnstc.
package process

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// ProcessInfo holds information about a managed process.
type ProcessInfo struct {
	Name    string    `json:"name"`
	PID     int       `json:"pid"`
	Binary  string    `json:"binary"`
	Args    []string  `json:"args"`
	Started time.Time `json:"started"`
}

// Manager handles process lifecycle.
type Manager struct {
	statePath string
	processes map[string]*ProcessInfo
	cmds      map[string]*exec.Cmd
	mu        sync.RWMutex
}

// NewManager creates a new process manager.
func NewManager(statePath string) *Manager {
	m := &Manager{
		statePath: statePath,
		processes: make(map[string]*ProcessInfo),
		cmds:      make(map[string]*exec.Cmd),
	}
	m.loadState()
	return m
}

// Start starts a process with the given name and command.
func (m *Manager) Start(name, binary string, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunningLocked(name) {
		return fmt.Errorf("process %s is already running", name)
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	info := &ProcessInfo{
		Name:    name,
		PID:     cmd.Process.Pid,
		Binary:  binary,
		Args:    args,
		Started: time.Now(),
	}

	m.processes[name] = info
	m.cmds[name] = cmd

	go m.monitor(name, cmd)

	return m.saveState()
}

// Stop stops a process by name.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopLocked(name)
}

func (m *Manager) stopLocked(name string) error {
	info, ok := m.processes[name]
	if !ok {
		return nil
	}

	process, err := os.FindProcess(info.PID)
	if err != nil {
		delete(m.processes, name)
		delete(m.cmds, name)
		return m.saveState()
	}

	if runtime.GOOS == "windows" {
		err = process.Kill()
	} else {
		err = process.Signal(syscall.SIGTERM)
		if err == nil {
			done := make(chan struct{})
			go func() {
				process.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				process.Kill()
			}
		}
	}

	delete(m.processes, name)
	delete(m.cmds, name)
	return m.saveState()
}

// StopAll stops all managed processes.
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name := range m.processes {
		if err := m.stopLocked(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// IsRunning checks if a process is running.
func (m *Manager) IsRunning(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunningLocked(name)
}

func (m *Manager) isRunningLocked(name string) bool {
	info, ok := m.processes[name]
	if !ok {
		return false
	}

	process, err := os.FindProcess(info.PID)
	if err != nil {
		return false
	}

	if runtime.GOOS != "windows" {
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}

	return true
}

// GetStatus returns status of all processes.
func (m *Manager) GetStatus() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]bool)
	for name := range m.processes {
		status[name] = m.isRunningLocked(name)
	}
	return status
}

// GetProcessInfo returns info about a specific process.
func (m *Manager) GetProcessInfo(name string) *ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, ok := m.processes[name]; ok {
		infoCopy := *info
		return &infoCopy
	}
	return nil
}

func (m *Manager) monitor(name string, cmd *exec.Cmd) {
	cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.processes, name)
	delete(m.cmds, name)
	m.saveState()
}

func (m *Manager) loadState() error {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Processes []*ProcessInfo `json:"processes"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	for _, info := range state.Processes {
		process, err := os.FindProcess(info.PID)
		if err != nil {
			continue
		}

		if runtime.GOOS != "windows" {
			if err := process.Signal(syscall.Signal(0)); err != nil {
				continue
			}
		}

		m.processes[info.Name] = info
	}

	return nil
}

func (m *Manager) saveState() error {
	dir := filepath.Dir(m.statePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	var state struct {
		Processes []*ProcessInfo `json:"processes"`
	}

	for _, info := range m.processes {
		state.Processes = append(state.Processes, info)
	}

	data, err := json.MarshalIndent(&state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.statePath, data, 0640)
}
