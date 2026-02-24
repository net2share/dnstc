// Package cmd provides the Cobra CLI for dnstc.
package cmd

import (
	"os"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/handlers"
	"github.com/net2share/dnstc/internal/ipc"
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

		config.MigrateConfigIfNeeded()

		// Try to connect to existing daemon
		if running, client := ipc.DetectDaemon(); running {
			engine.Set(client)
			menu.SetDaemonMode(true)
			menu.SetDaemonClient(client)
			defer func() {
				client.Close()
				engine.Set(nil)
			}()
		}
		// No daemon: engine.Get() == nil, TUI works in config-only mode

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
	handlers.AppVersion = version
	rootCmd.Version = version + " (built " + buildTime + ")"
}
