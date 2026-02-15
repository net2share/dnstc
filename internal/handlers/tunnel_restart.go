package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
)

func init() {
	actions.SetHandler(actions.ActionTunnelRestart, HandleTunnelRestart)
}

// HandleTunnelRestart restarts a tunnel.
func HandleTunnelRestart(ctx *actions.Context) error {
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

	beginProgress(ctx, fmt.Sprintf("Restart Tunnel: %s", tag))
	ctx.Output.Info(fmt.Sprintf("Restarting tunnel '%s'...", tag))

	if err := eng.RestartTunnel(tag); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to restart tunnel: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' restarted!", tag))
	endProgress(ctx)
	return nil
}
