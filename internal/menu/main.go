// Package menu provides the interactive menu for dnstc.
package menu

import (
	"errors"
	"fmt"
	"os"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
	"github.com/net2share/go-corelib/osdetect"
	"github.com/net2share/go-corelib/tui"
)

// errCancelled is returned when user cancels/backs out.
var errCancelled = errors.New("cancelled")

// Version and BuildTime are set by cmd package.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// daemonMode indicates the TUI is connected to an external daemon via IPC.
var daemonMode bool

// daemonClient holds the IPC client when connected to a daemon.
// Managed by recheckDaemon — closed when daemon disappears.
var daemonClient *ipc.Client

// SetDaemonMode sets whether the TUI is connected to a daemon.
func SetDaemonMode(v bool) { daemonMode = v }

// IsDaemonMode returns true if connected to a daemon.
func IsDaemonMode() bool { return daemonMode }

// SetDaemonClient stores the IPC client for daemon mode lifecycle management.
func SetDaemonClient(c *ipc.Client) { daemonClient = c }

// recheckDaemon detects if a daemon appeared or disappeared since last check,
// and switches the engine accordingly.
func recheckDaemon() {
	if daemonMode {
		// We're in daemon mode — verify daemon is still alive
		if daemonClient != nil {
			if _, err := daemonClient.Ping(); err == nil {
				return // still alive
			}
			// Daemon died — switch to nil engine
			daemonClient.Close()
			daemonClient = nil
		}
		daemonMode = false
		engine.Set(nil)
		return
	}

	// We're in local mode — check if a daemon appeared
	running, client := ipc.DetectDaemon()
	if !running {
		return
	}

	// Daemon appeared — switch to daemon mode
	daemonMode = true
	daemonClient = client
	engine.Set(client)
}

const dnstcBanner = `
    ____  _   _______  ______
   / __ \/ | / / ___/ /_  __/____
  / / / /  |/ /\__ \   / / / ___/
 / /_/ / /|  /___/ /  / / / /__
/_____/_/ |_//____/  /_/  \___/
`

// PrintBanner displays the dnstc banner with version info.
func PrintBanner() {
	tui.PrintBanner(tui.BannerConfig{
		AppName:   "DNS Tunnel Client",
		Version:   Version,
		BuildTime: BuildTime,
		ASCII:     dnstcBanner,
	})
}

// buildTunnelSummary builds a summary string for the main menu header.
func buildTunnelSummary() string {
	if !binaries.AreInstalled() {
		return "Not installed — run Install Binaries first"
	}

	eng := engine.Get()

	if eng == nil {
		// No daemon — load config from disk for tunnel count
		cfg, err := config.LoadOrDefault()
		if err != nil || len(cfg.Tunnels) == 0 {
			return "Disconnected"
		}
		return fmt.Sprintf("Disconnected | Tunnels: %d", len(cfg.Tunnels))
	}

	cfg := eng.GetConfig()
	total := len(cfg.Tunnels)
	if total == 0 {
		if daemonMode {
			return "[daemon]"
		}
		return ""
	}

	status := eng.Status()

	running := 0
	for _, ts := range status.Tunnels {
		if ts.Running {
			running++
		}
	}

	connState := "Disconnected"
	if running > 0 {
		connState = "Connected"
	}

	summary := fmt.Sprintf("%s | Tunnels: %d | Running: %d", connState, total, running)
	if status.GatewayAddr != "" {
		summary += fmt.Sprintf(" | Gateway: %s", status.GatewayAddr)
	}
	if status.Active != "" {
		summary += fmt.Sprintf(" | Active: %s", status.Active)
	}
	if daemonMode {
		summary += " | [daemon]"
	}
	return summary
}

// RunInteractive shows the main interactive menu.
func RunInteractive() error {
	PrintBanner()

	osInfo, err := osdetect.Detect()
	if err != nil {
		tui.PrintWarning("Could not detect OS: " + err.Error())
	} else {
		tui.PrintInfo(fmt.Sprintf("Detected OS: %s", osInfo.PrettyName))
	}

	arch := osdetect.GetArch()
	tui.PrintInfo(fmt.Sprintf("Architecture: %s", arch))

	return runMainMenu()
}

func runMainMenu() error {
	for {
		// Re-check for daemon each iteration
		recheckDaemon()

		header := buildTunnelSummary()

		var options []tui.MenuOption
		installed := binaries.AreInstalled()

		if installed {
			// Check live state for Connect/Disconnect
			eng := engine.Get()
			if eng != nil && eng.IsConnected() {
				options = append(options, tui.MenuOption{Label: "Disconnect", Value: "disconnect"})
			} else {
				options = append(options, tui.MenuOption{Label: "Connect", Value: "connect"})
			}

			options = append(options, tui.MenuOption{Label: "Tunnels →", Value: actions.ActionTunnel})
			options = append(options, tui.MenuOption{Label: "Configure →", Value: actions.ActionConfig})
			options = append(options, tui.MenuOption{Label: "Check Updates", Value: actions.ActionUpdate})
		} else {
			options = append(options, tui.MenuOption{Label: "Install Binaries", Value: actions.ActionInstall})
		}

		if config.IsInstalled() {
			options = append(options, tui.MenuOption{Label: "Uninstall", Value: actions.ActionUninstall})
		}
		options = append(options, tui.MenuOption{Label: "Exit", Value: "exit"})

		choice, err := tui.RunMenu(tui.MenuConfig{
			Header:  header,
			Title:   "DNS Tunnel Client",
			Options: options,
		})
		if err != nil {
			return err
		}

		if choice == "" || choice == "exit" {
			return nil
		}

		err = handleMainMenuChoice(choice)
		if errors.Is(err, errCancelled) {
			continue
		}
		if err != nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		}
	}
}

func handleMainMenuChoice(choice string) error {
	switch choice {
	case "connect":
		return handleConnect()
	case "disconnect":
		return handleDisconnect()
	case actions.ActionTunnel:
		return runTunnelMenu()
	case actions.ActionConfig:
		return RunSubmenu(actions.ActionConfig)
	case actions.ActionInstall:
		if err := RunAction(actions.ActionInstall); err != nil && err != errCancelled {
			return err
		}
		return nil
	case actions.ActionUpdate:
		if err := RunAction(actions.ActionUpdate); err != nil && err != errCancelled {
			return err
		}
		return nil
	case actions.ActionUninstall:
		if err := RunAction(actions.ActionUninstall); err != nil {
			if err == errCancelled {
				return errCancelled
			}
			return err
		}
		tui.EndSession()
		os.Exit(0)
	}
	return nil
}

func handleConnect() error {
	// Check if tunnels exist before forking a daemon
	cfg, err := config.LoadOrDefault()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if len(cfg.Tunnels) == 0 {
		_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "No tunnels configured. Add one first."})
		return errCancelled
	}

	pv := tui.NewProgressView("Connecting")
	pv.AddInfo("Starting daemon...")

	// Fork daemon if needed, get connected client
	client, err := ipc.EnsureDaemon()
	if err != nil {
		pv.AddError(fmt.Sprintf("Failed to start daemon: %v", err))
		pv.Done()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Set daemon mode
	daemonMode = true
	daemonClient = client
	engine.Set(client)

	// Start tunnels via IPC
	if err := client.Start(); err != nil {
		pv.AddError(fmt.Sprintf("Failed to connect: %v", err))
		pv.Done()
		return fmt.Errorf("failed to connect: %w", err)
	}

	status := client.Status()
	running := 0
	for _, ts := range status.Tunnels {
		if ts.Running {
			running++
		}
	}

	if running == 0 {
		client.Stop()
		client.Shutdown()
		client.Close()
		daemonMode = false
		daemonClient = nil
		engine.Set(nil)
		pv.AddError("No tunnels could be started")
		pv.Done()
		return fmt.Errorf("no tunnels could be started")
	}

	pv.AddSuccess(fmt.Sprintf("Connected — %d tunnel(s) running", running))
	pv.Done()
	return nil
}

func handleDisconnect() error {
	eng := engine.Get()
	if eng == nil {
		return fmt.Errorf("not connected")
	}

	// Stop tunnels via IPC
	eng.Stop()

	// Shutdown daemon process
	if daemonClient != nil {
		daemonClient.Shutdown()
		daemonClient.Close()
	}

	daemonMode = false
	daemonClient = nil
	engine.Set(nil)

	_ = tui.ShowMessage(tui.AppMessage{Type: "success", Message: "Disconnected"})
	return nil
}

// runTunnelMenu shows the tunnel submenu.
func runTunnelMenu() error {
	for {
		options := []tui.MenuOption{
			{Label: "Add", Value: actions.ActionTunnelAdd},
			{Label: "Import", Value: actions.ActionTunnelImport},
			{Label: "List →", Value: "list"},
			{Label: "Back", Value: "back"},
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Tunnels",
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		switch choice {
		case actions.ActionTunnelAdd:
			if err := RunAction(actions.ActionTunnelAdd); err != nil {
				if err != errCancelled {
					_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
				}
			} else {
				if eng := engine.Get(); eng != nil {
					eng.ReloadConfig()
				}
			}
		case actions.ActionTunnelImport:
			if err := RunAction(actions.ActionTunnelImport); err != nil {
				if err != errCancelled {
					_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
				}
			} else {
				if eng := engine.Get(); eng != nil {
					eng.ReloadConfig()
				}
			}
		case "list":
			_ = runTunnelListMenu()
		}
	}
}

// runTunnelListMenu shows all tunnels and allows selecting one to manage.
func runTunnelListMenu() error {
	for {
		eng := engine.Get()

		// Load config — from engine if available, otherwise from disk
		var cfg *config.Config
		var status *engine.Status

		if eng != nil {
			cfg = eng.GetConfig()
			status = eng.Status()
		} else {
			var err error
			cfg, err = config.LoadOrDefault()
			if err != nil {
				_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: "Failed to load config"})
				return errCancelled
			}
			// Build empty status — all tunnels stopped
			status = &engine.Status{
				Active:  cfg.Route.Active,
				Tunnels: make(map[string]*engine.TunnelStatus),
			}
			for _, tc := range cfg.Tunnels {
				status.Tunnels[tc.Tag] = &engine.TunnelStatus{
					Tag:    tc.Tag,
					Active: tc.Tag == cfg.Route.Active,
				}
			}
		}

		if len(cfg.Tunnels) == 0 {
			_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "No tunnels configured. Add one first."})
			return errCancelled
		}

		var options []tui.MenuOption
		for _, tc := range cfg.Tunnels {
			ts := status.Tunnels[tc.Tag]
			statusIcon := "○"
			if ts != nil && ts.Running {
				statusIcon = "●"
			}
			transportName := config.GetTransportTypeDisplayName(tc.Transport)
			backendName := config.GetBackendTypeDisplayName(tc.Backend)
			portLabel := ""
			if tc.Port > 0 {
				portLabel = fmt.Sprintf(" :%d", tc.Port)
			}
			label := fmt.Sprintf("%s %s (%s/%s → %s%s)", statusIcon, tc.Tag, transportName, backendName, tc.Domain, portLabel)
			if tc.Tag == cfg.Route.Active {
				label += " [active]"
			}
			options = append(options, tui.MenuOption{Label: label, Value: tc.Tag})
		}
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		selected, err := tui.RunMenu(tui.MenuConfig{
			Title:   "Select Tunnel",
			Options: options,
		})
		if err != nil || selected == "" || selected == "back" {
			return errCancelled
		}

		_ = runTunnelManageMenu(selected)
	}
}

// runTunnelManageMenu shows management options for a specific tunnel.
func runTunnelManageMenu(tag string) error {
	for {
		eng := engine.Get()

		// Load config and status — from engine or disk
		var cfg *config.Config
		var status *engine.Status

		if eng != nil {
			cfg = eng.GetConfig()
			status = eng.Status()
		} else {
			var err error
			cfg, err = config.LoadOrDefault()
			if err != nil {
				return errCancelled
			}
			status = &engine.Status{
				Active:  cfg.Route.Active,
				Tunnels: make(map[string]*engine.TunnelStatus),
			}
			for _, tc := range cfg.Tunnels {
				status.Tunnels[tc.Tag] = &engine.TunnelStatus{
					Tag:    tc.Tag,
					Active: tc.Tag == cfg.Route.Active,
				}
			}
		}

		tc := cfg.GetTunnelByTag(tag)
		if tc == nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: fmt.Sprintf("Tunnel '%s' not found", tag)})
			return nil
		}

		ts := status.Tunnels[tag]

		statusStr := "Stopped"
		if ts != nil && ts.Running {
			statusStr = "Running"
		}

		var options []tui.MenuOption
		options = append(options, tui.MenuOption{Label: "Status", Value: "status"})

		if ts == nil || !ts.Active {
			options = append(options, tui.MenuOption{Label: "Activate", Value: "activate"})
		}

		options = append(options,
			tui.MenuOption{Label: "Remove", Value: "remove"},
			tui.MenuOption{Label: "Back", Value: "back"},
		)

		transportDisplay := config.GetTransportTypeDisplayName(tc.Transport)
		backendDisplay := config.GetBackendTypeDisplayName(tc.Backend)

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:       fmt.Sprintf("%s (%s)", tag, statusStr),
			Description: fmt.Sprintf("%s/%s → %s", transportDisplay, backendDisplay, tc.Domain),
			Options:     options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		actionID := "tunnel." + choice
		if err := runTunnelAction(actionID, tag); err != nil {
			if err == errCancelled {
				continue
			}
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		} else if choice == "remove" {
			// Reload engine config after removing a tunnel
			if eng := engine.Get(); eng != nil {
				eng.ReloadConfig()
			}
			return errCancelled
		}
	}
}

// runTunnelAction runs a tunnel action with the given tag as argument.
func runTunnelAction(actionID, tunnelTag string) error {
	switch actionID {
	case actions.ActionTunnelStatus,
		actions.ActionTunnelRemove, actions.ActionTunnelActivate:
		return runActionWithArgs(actionID, []string{tunnelTag})
	default:
		return RunAction(actionID)
	}
}
