package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
)

func init() {
	actions.SetHandler(actions.ActionTunnelStop, HandleTunnelStop)
}

// HandleTunnelStop stops a tunnel.
func HandleTunnelStop(ctx *actions.Context) error {
	eng, err := RequireEngine()
	if err != nil {
		return err
	}

	tag, err := RequireTag(ctx)
	if err != nil {
		return err
	}

	cfg := eng.GetConfig()
	if cfg.GetTunnelByTag(tag) == nil {
		return actions.TunnelNotFoundError(tag)
	}

	beginProgress(ctx, fmt.Sprintf("Stop Tunnel: %s", tag))
	ctx.Output.Info(fmt.Sprintf("Stopping tunnel '%s'...", tag))

	if err := eng.StopTunnel(tag); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to stop tunnel: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' stopped!", tag))
	endProgress(ctx)
	return nil
}
