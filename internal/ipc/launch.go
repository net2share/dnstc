package ipc

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/net2share/dnstc/internal/config"
)

// EnsureDaemon returns a connected client to a running daemon.
// If no daemon is running, it forks one in the background and waits for it to become ready.
func EnsureDaemon() (*Client, error) {
	// If daemon already running, return connected client
	if running, client := DetectDaemon(); running {
		return client, nil
	}

	// Fork a new daemon process
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to determine executable path: %w", err)
	}

	if err := config.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("failed to create config dirs: %w", err)
	}

	logFile, err := os.OpenFile(config.DaemonLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open daemon log: %w", err)
	}

	cmd := exec.Command(exe, "daemon", "run")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to fork daemon: %w", err)
	}

	// Detach — don't wait for the child
	go func() {
		cmd.Wait()
		logFile.Close()
	}()

	// Poll for daemon readiness
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if running, client := DetectDaemon(); running {
			return client, nil
		}
	}

	return nil, fmt.Errorf("daemon did not start within 10 seconds (check %s)", config.DaemonLogPath())
}
