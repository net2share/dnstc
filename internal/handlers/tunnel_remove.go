package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
)

func init() {
	actions.SetHandler(actions.ActionTunnelRemove, HandleTunnelRemove)
}

// HandleTunnelRemove removes a tunnel.
func HandleTunnelRemove(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx)
	if err != nil {
		return err
	}

	if cfg.GetTunnelByTag(tag) == nil {
		return actions.TunnelNotFoundError(tag)
	}

	beginProgress(ctx, fmt.Sprintf("Remove Tunnel: %s", tag))

	totalSteps := 3
	currentStep := 0

	// Step 1: Stop if running (via engine if available)
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Stopping tunnel...")
	if eng := engine.Get(); eng != nil {
		eng.StopTunnel(tag)
	}
	ctx.Output.Status("Tunnel stopped")

	// Step 2: Remove from config
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing configuration...")
	var tunnels []config.TunnelConfig
	for _, tc := range cfg.Tunnels {
		if tc.Tag != tag {
			tunnels = append(tunnels, tc)
		}
	}
	cfg.Tunnels = tunnels

	if cfg.Route.Active == tag {
		cfg.Route.Active = ""
	}

	// Step 3: Save
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Saving configuration...")
	if err := cfg.Save(); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to save config: %w", err))
	}
	ctx.Output.Status("Configuration saved")

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' removed!", tag))
	endProgress(ctx)
	return nil
}
