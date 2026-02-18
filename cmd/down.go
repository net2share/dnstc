package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the dnstc service",
	Long:  "Stop the dnstc systemd service if it is running.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("service management is only supported on Linux")
		}

		out, err := exec.Command("systemctl", "is-active", serviceName).Output()
		if err != nil || string(out) != "active\n" {
			fmt.Println("dnstc service is not running.")
			return nil
		}

		fmt.Println("Stopping dnstc service...")
		if err := runSystemctl("stop", serviceName); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}

		fmt.Println("Stopped.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
