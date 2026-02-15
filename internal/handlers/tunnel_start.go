package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
)

func init() {
	actions.SetHandler(actions.ActionTunnelStart, HandleTunnelStart)
}

// HandleTunnelStart starts a tunnel.
func HandleTunnelStart(ctx *actions.Context) error {
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

	beginProgress(ctx, fmt.Sprintf("Start Tunnel: %s", tag))
	ctx.Output.Info(fmt.Sprintf("Starting tunnel '%s'...", tag))

	if err := eng.StartTunnel(tag); err != nil {
		return failProgress(ctx, fmt.Errorf("failed to start tunnel: %w", err))
	}

	ctx.Output.Success(fmt.Sprintf("Tunnel '%s' started!", tag))
	endProgress(ctx)
	return nil
}
