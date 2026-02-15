// Package menu provides the interactive menu for dnstc.
package menu

import (
	"errors"
	"fmt"
	"os"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
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
	eng := engine.Get()
	if eng == nil {
		return ""
	}

	cfg := eng.GetConfig()
	total := len(cfg.Tunnels)
	if total == 0 {
		return ""
	}

	status := eng.Status()

	running := 0
	for _, ts := range status.Tunnels {
		if ts.Running {
			running++
		}
	}

	summary := fmt.Sprintf("Tunnels: %d | Running: %d", total, running)
	if status.GatewayAddr != "" {
		summary += fmt.Sprintf(" | Gateway: %s", status.GatewayAddr)
	}
	if status.Active != "" {
		summary += fmt.Sprintf(" | Active: %s", status.Active)
	}
	return summary
}

// RunInteractive shows the main interactive menu.
// The engine must be set via engine.Set() before calling this.
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
		header := buildTunnelSummary()

		var options []tui.MenuOption
		options = append(options, tui.MenuOption{Label: "Tunnels →", Value: actions.ActionTunnel})
		options = append(options, tui.MenuOption{Label: "Configure →", Value: actions.ActionConfig})
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
	case actions.ActionTunnel:
		return runTunnelMenu()
	case actions.ActionConfig:
		return RunSubmenu(actions.ActionConfig)
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

// runTunnelMenu shows the tunnel submenu.
func runTunnelMenu() error {
	for {
		options := []tui.MenuOption{
			{Label: "Add", Value: actions.ActionTunnelAdd},
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
				// Reload engine config after adding a tunnel
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
		if eng == nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: "Engine not running"})
			return errCancelled
		}

		cfg := eng.GetConfig()
		if len(cfg.Tunnels) == 0 {
			_ = tui.ShowMessage(tui.AppMessage{Type: "info", Message: "No tunnels configured. Add one first."})
			return errCancelled
		}

		status := eng.Status()

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

		if err := runTunnelManageMenu(selected); err != errCancelled {
			tui.WaitForEnter()
		}
	}
}

// runTunnelManageMenu shows management options for a specific tunnel.
func runTunnelManageMenu(tag string) error {
	for {
		eng := engine.Get()
		if eng == nil {
			return errCancelled
		}

		cfg := eng.GetConfig()
		tc := cfg.GetTunnelByTag(tag)
		if tc == nil {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: fmt.Sprintf("Tunnel '%s' not found", tag)})
			return nil
		}

		status := eng.Status()
		ts := status.Tunnels[tag]

		statusStr := "Stopped"
		isRunning := ts != nil && ts.Running
		if isRunning {
			statusStr = "Running"
		}

		var options []tui.MenuOption
		options = append(options, tui.MenuOption{Label: "Status", Value: "status"})

		if isRunning {
			options = append(options,
				tui.MenuOption{Label: "Restart", Value: "restart"},
				tui.MenuOption{Label: "Stop", Value: "stop"},
			)
		} else {
			options = append(options, tui.MenuOption{Label: "Start", Value: "start"})
		}

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
		} else {
			if choice == "remove" {
				// Reload engine config after removing a tunnel
				if eng := engine.Get(); eng != nil {
					eng.ReloadConfig()
				}
				return errCancelled
			}
			if !isInfoViewAction(actionID) {
				tui.WaitForEnter()
			}
		}
	}
}

// runTunnelAction runs a tunnel action with the given tag as argument.
func runTunnelAction(actionID, tunnelTag string) error {
	switch actionID {
	case actions.ActionTunnelStatus, actions.ActionTunnelStart,
		actions.ActionTunnelStop, actions.ActionTunnelRestart,
		actions.ActionTunnelRemove, actions.ActionTunnelActivate:
		return runActionWithArgs(actionID, []string{tunnelTag})
	default:
		return RunAction(actionID)
	}
}
