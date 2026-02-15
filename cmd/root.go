// Package cmd provides the Cobra CLI for dnstc.
package cmd

import (
	"fmt"
	"os"

	// Import handlers to register them with actions
	_ "github.com/net2share/dnstc/internal/handlers"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/menu"
	"github.com/net2share/go-corelib/tui"
	"github.com/spf13/cobra"
)

// Version and BuildTime are set at build time.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "dnstc",
	Short: "DNS Tunnel Client",
	Long:  "DNS Tunnel Client - https://github.com/net2share/dnstc",
	RunE: func(cmd *cobra.Command, args []string) error {
		menu.Version = Version
		menu.BuildTime = BuildTime
		tui.SetAppInfo("dnstc", Version, BuildTime)
		tui.BeginSession()
		defer tui.EndSession()

		// Load config and start engine
		config.MigrateConfigIfNeeded()
		cfg, err := config.LoadOrDefault()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		eng := engine.New(cfg)
		engine.Set(eng)
		defer func() {
			eng.Stop()
			engine.Set(nil)
		}()

		return menu.RunInteractive()
	},
}

func init() {
	rootCmd.Version = Version

	// Register all action-based commands
	RegisterActionsWithRoot(rootCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetVersionInfo sets version information for the CLI.
func SetVersionInfo(version, buildTime string) {
	Version = version
	BuildTime = buildTime
	rootCmd.Version = version + " (built " + buildTime + ")"
}
