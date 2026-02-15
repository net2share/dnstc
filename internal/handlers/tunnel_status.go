package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
)

func init() {
	actions.SetHandler(actions.ActionTunnelStatus, HandleTunnelStatus)
}

// HandleTunnelStatus shows status for a specific tunnel.
func HandleTunnelStatus(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx)
	if err != nil {
		return err
	}

	tc := cfg.GetTunnelByTag(tag)
	if tc == nil {
		return actions.TunnelNotFoundError(tag)
	}

	// Check live status from engine if running
	statusStr := "Stopped"
	isActive := tc.Tag == cfg.Route.Active
	if eng := engine.Get(); eng != nil {
		status := eng.Status()
		ts := status.Tunnels[tag]
		if ts != nil && ts.Running {
			statusStr = fmt.Sprintf("Running (port %d)", ts.Port)
		}
		isActive = ts != nil && ts.Active
	}

	activeStr := "No"
	if isActive {
		activeStr = "Yes"
	}

	portStr := "auto"
	if tc.Port > 0 {
		portStr = fmt.Sprintf("%d", tc.Port)
	}

	infoCfg := actions.InfoConfig{
		Title: fmt.Sprintf("Tunnel: %s", tag),
		Sections: []actions.InfoSection{
			{
				Rows: []actions.InfoRow{
					{Key: "Transport", Value: config.GetTransportTypeDisplayName(tc.Transport)},
					{Key: "Backend", Value: config.GetBackendTypeDisplayName(tc.Backend)},
					{Key: "Domain", Value: tc.Domain},
					{Key: "Port", Value: portStr},
					{Key: "Status", Value: statusStr},
					{Key: "Active", Value: activeStr},
				},
			},
		},
	}

	if tc.Resolver != "" {
		infoCfg.Sections[0].Rows = append(infoCfg.Sections[0].Rows,
			actions.InfoRow{Key: "Resolver", Value: tc.Resolver})
	}

	if ctx.IsInteractive {
		return ctx.Output.ShowInfo(infoCfg)
	}

	// CLI mode
	lines := []string{
		fmt.Sprintf("Tag: %s", tag),
		fmt.Sprintf("Transport: %s", config.GetTransportTypeDisplayName(tc.Transport)),
		fmt.Sprintf("Backend: %s", config.GetBackendTypeDisplayName(tc.Backend)),
		fmt.Sprintf("Domain: %s", tc.Domain),
		fmt.Sprintf("Port: %s", portStr),
		fmt.Sprintf("Status: %s", statusStr),
		fmt.Sprintf("Active: %s", activeStr),
	}
	if tc.Resolver != "" {
		lines = append(lines, fmt.Sprintf("Resolver: %s", tc.Resolver))
	}
	ctx.Output.Box("Tunnel Status", lines)
	return nil
}
