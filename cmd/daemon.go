package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
	"github.com/net2share/dnstc/internal/process"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background daemon",
}

// daemonRunCmd is the hidden foreground process used by daemon start and systemd.
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
	Short: "Start the daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !binaries.AreInstalled() {
			return fmt.Errorf("binaries not installed — run 'dnstc install' first")
		}

		// Check if already running
		if running, client := ipc.DetectDaemon(); running {
			status := client.Status()
			client.Close()
			runCount := 0
			for _, ts := range status.Tunnels {
				if ts.Running {
					runCount++
				}
			}
			fmt.Printf("Daemon already running (%d tunnel(s) active)\n", runCount)
			return nil
		}

		fmt.Println("Starting daemon...")
		client, err := ipc.EnsureDaemon()
		if err != nil {
			return err
		}
		defer client.Close()

		// Check if there are tunnels to start
		cfg := client.GetConfig()
		if len(cfg.Tunnels) == 0 {
			fmt.Println("Daemon started (no tunnels configured)")
			return nil
		}

		// Start tunnels via IPC
		if err := client.Start(); err != nil {
			return fmt.Errorf("failed to start tunnels: %w", err)
		}

		// Print status
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
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		if running, client := ipc.DetectDaemon(); running {
			fmt.Println("Stopping daemon...")
			client.Stop()
			client.Shutdown()
			client.Close()
			fmt.Println("Stopped.")
			return nil
		}

		// No daemon — check for orphan processes
		mgr := process.NewManager(config.StatePath())
		status := mgr.GetStatus()

		orphans := 0
		for _, alive := range status {
			if alive {
				orphans++
			}
		}

		if orphans == 0 {
			fmt.Println("Nothing is running.")
			return nil
		}

		fmt.Printf("Stopping %d orphan process(es)...\n", orphans)
		mgr.StopAll()
		fmt.Println("Stopped.")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			if status.DNSProxyAddr != "" {
				fmt.Printf("DNS Proxy: %s\n", status.DNSProxyAddr)
			}
			return nil
		}

		// No daemon — check for orphans
		mgr := process.NewManager(config.StatePath())
		status := mgr.GetStatus()

		orphans := 0
		for name, alive := range status {
			if alive {
				orphans++
				info := mgr.GetProcessInfo(name)
				if info != nil {
					fmt.Printf("  orphan: %s (pid %d)\n", name, info.PID)
				}
			}
		}

		if orphans == 0 {
			fmt.Println("No daemon running.")
		} else {
			fmt.Printf("No daemon running, but %d orphan process(es) found.\n", orphans)
			fmt.Println("Run 'dnstc daemon stop' to clean them up.")
		}
		return nil
	},
}

const systemdUnit = `[Unit]
Description=DNS Tunnel Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
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
		if !binaries.AreInstalled() {
			return fmt.Errorf("binaries not installed — run 'dnstc install' first")
		}
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

		unit := fmt.Sprintf(systemdUnit, binPath)
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
		fmt.Println("Start with: sudo systemctl start dnstc")
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

func init() {
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonEnableCmd)
	daemonCmd.AddCommand(daemonDisableCmd)
	rootCmd.AddCommand(daemonCmd)
}
