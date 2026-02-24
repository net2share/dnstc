package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	appName = "dnstc"
)

// ConfigDir returns the platform-specific configuration directory.
func ConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", appName)
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), appName)
	default: // linux and others
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, appName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", appName)
	}
}

// BinDir returns the platform-specific binary directory.
func BinDir() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", appName, "bin")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), appName, "bin")
	default: // linux and others
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			return filepath.Join(xdgData, appName, "bin")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", appName, "bin")
	}
}

// Path returns the full path to the config file.
func Path() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// OldConfigPath returns the path to the old YAML config file.
func OldConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// StatePath returns the path to the runtime state file.
func StatePath() string {
	return filepath.Join(ConfigDir(), "state.json")
}

// SocketPath returns the path to the daemon IPC socket.
func SocketPath() string {
	return filepath.Join(ConfigDir(), "engine.sock")
}

// DaemonLogPath returns the path to the daemon log file.
func DaemonLogPath() string {
	return filepath.Join(ConfigDir(), "daemon.log")
}

// VersionsPath returns the path to the binary version manifest.
func VersionsPath() string {
	return filepath.Join(ConfigDir(), "versions.json")
}

// EnsureDirs creates the config and bin directories if they don't exist.
func EnsureDirs() error {
	if err := os.MkdirAll(ConfigDir(), 0750); err != nil {
		return err
	}
	return os.MkdirAll(BinDir(), 0750)
}

// IsInstalled checks if dnstc has any installed components.
func IsInstalled() bool {
	configDir := ConfigDir()
	if entries, err := os.ReadDir(configDir); err == nil && len(entries) > 0 {
		return true
	}

	binDir := BinDir()
	if entries, err := os.ReadDir(binDir); err == nil && len(entries) > 0 {
		return true
	}

	dataDir := filepath.Dir(binDir)
	if entries, err := os.ReadDir(dataDir); err == nil && len(entries) > 0 {
		return true
	}

	return false
}
