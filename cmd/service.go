package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const serviceUnit = `[Unit]
Description=DNS Tunnel Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s up
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

const serviceName = "dnstc"
const unitPath = "/etc/systemd/system/dnstc.service"

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage dnstc systemd service (Linux only)",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install dnstc as a systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("service management is only supported on Linux")
		}

		if os.Geteuid() != 0 {
			return fmt.Errorf("root privileges required; run with sudo")
		}

		// Find the dnstc binary path
		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine binary path: %w", err)
		}
		binPath, err = filepath.Abs(binPath)
		if err != nil {
			return fmt.Errorf("failed to resolve binary path: %w", err)
		}

		unit := fmt.Sprintf(serviceUnit, binPath)
		if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
			return fmt.Errorf("failed to write unit file: %w", err)
		}

		// Reload and enable
		if err := runSystemctl("daemon-reload"); err != nil {
			return err
		}
		if err := runSystemctl("enable", serviceName); err != nil {
			return err
		}

		fmt.Println("Service installed and enabled.")
		fmt.Println("Start with: sudo systemctl start dnstc")

		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove dnstc systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("service management is only supported on Linux")
		}

		if os.Geteuid() != 0 {
			return fmt.Errorf("root privileges required; run with sudo")
		}

		// Stop and disable
		runSystemctl("stop", serviceName)
		runSystemctl("disable", serviceName)

		if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove unit file: %w", err)
		}

		runSystemctl("daemon-reload")

		fmt.Println("Service removed.")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show dnstc service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("service management is only supported on Linux")
		}

		out, err := exec.Command("systemctl", "status", serviceName).CombinedOutput()
		if err != nil {
			// systemctl status returns non-zero for inactive services
			fmt.Print(string(out))
			return nil
		}
		fmt.Print(string(out))
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
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	rootCmd.AddCommand(serviceCmd)
}
