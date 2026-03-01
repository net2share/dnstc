package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background daemon",
}

// daemonRunCmd is the hidden foreground process used by systemd ExecStart.
var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run the daemon in the foreground",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !binaries.AreInstalled() {
			return fmt.Errorf("binaries not installed — run 'dnstc install' first")
		}

		// Check for existing daemon via IPC
		if running, client := ipc.DetectDaemon(); running {
			client.Close()
			return fmt.Errorf("daemon is already running (socket: %s)", config.SocketPath())
		}

		// Load config
		config.MigrateConfigIfNeeded()
		cfg, err := config.LoadOrDefault()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create engine — stop any orphan processes from a previous session
		eng := engine.New(cfg)
		eng.Stop()
		engine.Set(eng)
		defer engine.Set(nil)

		// Start IPC server first so clients can connect immediately
		socketPath := config.SocketPath()
		srv := ipc.NewServer(socketPath, Version, eng)
		if err := srv.Start(); err != nil {
			return fmt.Errorf("failed to start IPC server: %w", err)
		}
		defer srv.Stop()

		// Auto-start tunnels so they come up after reboot
		if err := eng.Start(); err != nil {
			fmt.Printf("Warning: failed to auto-start tunnels: %v\n", err)
		}

		fmt.Printf("Daemon ready (socket: %s)\n", socketPath)

		// Wait for signal or shutdown request
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-sig:
		case <-srv.ShutdownCh:
		}

		fmt.Println("\nShutting down...")
		eng.Stop()
		fmt.Println("Stopped.")

		return nil
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon and tunnels",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If daemon already running, start tunnels via IPC
		if running, client := ipc.DetectDaemon(); running {
			return startTunnels(client)
		}

		// No daemon — try systemd on Linux
		if runtime.GOOS == "linux" {
			if _, err := os.Stat(systemdUnitPath); err == nil {
				fmt.Println("Starting service...")
				if err := runSystemctl("start", systemdServiceName); err != nil {
					return fmt.Errorf("failed to start service: %w", err)
				}

				// Poll IPC for readiness
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					time.Sleep(200 * time.Millisecond)
					if running, client := ipc.DetectDaemon(); running {
						return startTunnels(client)
					}
				}
				return fmt.Errorf("daemon did not become ready within 10s — check 'journalctl -u dnstc'")
			}
		}

		return fmt.Errorf("no daemon running — start with 'dnstc daemon run' or install the service with 'sudo dnstc daemon enable'")
	},
}

// startTunnels starts tunnels on a connected daemon and prints status.
func startTunnels(client *ipc.Client) error {
	defer client.Close()

	cfg := client.GetConfig()
	if len(cfg.Tunnels) == 0 {
		fmt.Println("Daemon running (no tunnels configured)")
		return nil
	}

	if err := client.Start(); err != nil {
		return fmt.Errorf("failed to start tunnels: %w", err)
	}

	status := client.Status()
	runCount := 0
	for _, ts := range status.Tunnels {
		if ts.Running {
			runCount++
			fmt.Printf("  tunnel %s running on :%d\n", ts.Tag, ts.Port)
		}
	}
	if status.GatewayAddr != "" {
		fmt.Printf("  gateway: %s\n", status.GatewayAddr)
	}
	fmt.Printf("Started (%d tunnel(s) running)\n", runCount)
	return nil
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try IPC shutdown first (daemon exits cleanly, Restart=on-failure won't restart)
		if running, client := ipc.DetectDaemon(); running {
			fmt.Println("Stopping daemon...")
			client.Stop()
			client.Shutdown()
			client.Close()
			fmt.Println("Stopped.")
			return nil
		}

		// Fallback: check if systemd service is active (e.g. running as different user)
		if runtime.GOOS == "linux" && isServiceActive() {
			fmt.Println("Stopping service via systemctl...")
			if err := runSystemctl("stop", systemdServiceName); err != nil {
				return fmt.Errorf("failed to stop service: %w", err)
			}
			fmt.Println("Stopped.")
			return nil
		}

		fmt.Println("Nothing is running.")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try IPC for detailed status
		if running, client := ipc.DetectDaemon(); running {
			status := client.Status()
			client.Close()

			runCount := 0
			for _, ts := range status.Tunnels {
				if ts.Running {
					runCount++
				}
			}

			fmt.Printf("Daemon running — %d/%d tunnel(s) active\n", runCount, len(status.Tunnels))
			for _, ts := range status.Tunnels {
				state := "stopped"
				if ts.Running {
					state = fmt.Sprintf("running :%d", ts.Port)
				}
				active := ""
				if ts.Active {
					active = " [active]"
				}
				fmt.Printf("  %s: %s%s\n", ts.Tag, state, active)
			}
			if status.GatewayAddr != "" {
				fmt.Printf("Gateway: %s\n", status.GatewayAddr)
			}
			return nil
		}

		// No IPC — check systemd service state
		if runtime.GOOS == "linux" {
			if isServiceActive() {
				fmt.Println("Service is active but IPC is not responding.")
				fmt.Println("Check logs: journalctl -u dnstc")
				return nil
			}
			if _, err := os.Stat(systemdUnitPath); os.IsNotExist(err) {
				fmt.Println("No daemon running.")
				fmt.Println("Install the service: sudo dnstc daemon enable")
				return nil
			}
		}

		fmt.Println("No daemon running.")
		return nil
	},
}

const systemdUnit = `[Unit]
Description=DNS Tunnel Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
ExecStart=%s daemon run
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

const (
	systemdServiceName = "dnstc"
	systemdUnitPath    = "/etc/systemd/system/dnstc.service"
)

var daemonEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Install and enable the systemd service (Linux only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("service management is only supported on Linux")
		}
		if os.Geteuid() != 0 {
			return fmt.Errorf("root privileges required; run with sudo")
		}

		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine binary path: %w", err)
		}
		binPath, err = filepath.Abs(binPath)
		if err != nil {
			return fmt.Errorf("failed to resolve binary path: %w", err)
		}

		// Resolve the invoking user (sudo sets SUDO_USER)
		username := os.Getenv("SUDO_USER")
		if username == "" {
			u, err := user.Current()
			if err != nil {
				return fmt.Errorf("could not determine current user: %w", err)
			}
			username = u.Username
		}

		unit := fmt.Sprintf(systemdUnit, username, binPath)
		if err := os.WriteFile(systemdUnitPath, []byte(unit), 0644); err != nil {
			return fmt.Errorf("failed to write unit file: %w", err)
		}

		if err := runSystemctl("daemon-reload"); err != nil {
			return err
		}
		if err := runSystemctl("enable", systemdServiceName); err != nil {
			return err
		}

		fmt.Println("Service installed and enabled.")
		fmt.Println("Start with: dnstc daemon start")
		return nil
	},
}

var daemonDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable and remove the systemd service (Linux only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("service management is only supported on Linux")
		}
		if os.Geteuid() != 0 {
			return fmt.Errorf("root privileges required; run with sudo")
		}

		runSystemctl("stop", systemdServiceName)
		runSystemctl("disable", systemdServiceName)

		if err := os.Remove(systemdUnitPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove unit file: %w", err)
		}

		runSystemctl("daemon-reload")

		fmt.Println("Service removed.")
		return nil
	},
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %v failed: %w", args, err)
	}
	return nil
}

// isServiceActive checks if the systemd service is currently active.
func isServiceActive() bool {
	return exec.Command("systemctl", "is-active", "--quiet", systemdServiceName).Run() == nil
}

func init() {
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonEnableCmd)
	daemonCmd.AddCommand(daemonDisableCmd)
	rootCmd.AddCommand(daemonCmd)
}
