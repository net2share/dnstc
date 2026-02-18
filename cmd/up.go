package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start all tunnels and gateway (headless)",
	Long:  "Start all enabled tunnels and the gateway, running in the foreground until interrupted.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load or migrate config
		config.MigrateConfigIfNeeded()
		cfg, err := config.LoadOrDefault()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Tunnels) == 0 {
			return fmt.Errorf("no tunnels configured; add one with: dnstc tunnel add")
		}

		eng := engine.New(cfg)
		engine.Set(eng)
		defer engine.Set(nil)

		fmt.Println("Starting dnstc engine...")

		if err := eng.Start(); err != nil {
			return fmt.Errorf("failed to start engine: %w", err)
		}

		status := eng.Status()
		running := 0
		for _, ts := range status.Tunnels {
			if ts.Running {
				running++
				fmt.Printf("  tunnel %s running on :%d\n", ts.Tag, ts.Port)
			}
		}

		if status.DNSProxyAddr != "" {
			fmt.Printf("  dns proxy listening on %s\n", status.DNSProxyAddr)
		}

		if status.GatewayAddr != "" {
			fmt.Printf("  gateway listening on %s → active tunnel: %s\n", status.GatewayAddr, status.Active)
		}

		if running == 0 {
			eng.Stop()
			return fmt.Errorf("no tunnels could be started")
		}

		fmt.Println("Press Ctrl+C to stop.")

		// Wait for interrupt
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		fmt.Println("\nShutting down...")
		eng.Stop()
		fmt.Println("Stopped.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
