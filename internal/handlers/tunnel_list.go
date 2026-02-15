package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
)

func init() {
	actions.SetHandler(actions.ActionTunnelList, HandleTunnelList)
}

// HandleTunnelList lists all configured tunnels.
func HandleTunnelList(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Tunnels) == 0 {
		ctx.Output.Info("No tunnels configured. Use 'dnstc tunnel add' to create one.")
		return nil
	}

	// Use engine for live status if available
	var tunnelRunning map[string]bool
	if eng := engine.Get(); eng != nil {
		status := eng.Status()
		tunnelRunning = make(map[string]bool)
		for tag, ts := range status.Tunnels {
			tunnelRunning[tag] = ts.Running
		}
	}

	headers := []string{"TAG", "TRANSPORT", "BACKEND", "DOMAIN", "PORT", "STATUS"}
	var rows [][]string

	for _, tc := range cfg.Tunnels {
		statusStr := "Stopped"
		if tunnelRunning != nil && tunnelRunning[tc.Tag] {
			statusStr = "Running"
		}

		portStr := "auto"
		if tc.Port > 0 {
			portStr = fmt.Sprintf("%d", tc.Port)
		}

		marker := ""
		if tc.Tag == cfg.Route.Active {
			marker = " *"
		}

		rows = append(rows, []string{
			tc.Tag + marker,
			config.GetTransportTypeDisplayName(tc.Transport),
			config.GetBackendTypeDisplayName(tc.Backend),
			tc.Domain,
			portStr,
			statusStr,
		})
	}

	ctx.Output.Table(headers, rows)
	ctx.Output.Println("\n* = active tunnel")
	return nil
}
