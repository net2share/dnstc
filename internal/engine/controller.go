package engine

import "github.com/net2share/dnstc/internal/config"

// EngineController defines the interface for controlling the engine.
// Both *Engine (local) and the IPC client implement this.
type EngineController interface {
	Start() error
	Stop() error
	StartTunnel(tag string) error
	StopTunnel(tag string) error
	RestartTunnel(tag string) error
	ActivateTunnel(tag string) error
	Status() *Status
	GetConfig() *config.Config
	ReloadConfig() error
	IsConnected() bool
}
