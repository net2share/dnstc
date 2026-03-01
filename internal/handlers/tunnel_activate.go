package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
)

func init() {
	actions.SetHandler(actions.ActionTunnelActivate, HandleTunnelActivate)
}

// HandleTunnelActivate sets a tunnel as the active route.
func HandleTunnelActivate(ctx *actions.Context) error {
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

	if cfg.Route.Active == tag {
		ctx.Output.Info(fmt.Sprintf("Tunnel '%s' is already active", tag))
		return nil
	}

	// If engine is running locally, use it (updates in-memory config + saves).
	// Otherwise, try IPC to a running daemon.
	// Fallback: update config on disk only.
	if eng := engine.Get(); eng != nil {
		if err := eng.ActivateTunnel(tag); err != nil {
			return fmt.Errorf("failed to activate tunnel: %w", err)
		}
	} else if running, client := ipc.DetectDaemon(); running {
		defer client.Close()
		if err := client.ActivateTunnel(tag); err != nil {
			return fmt.Errorf("failed to activate tunnel: %w", err)
		}
	} else {
		cfg.Route.Active = tag
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	ctx.Output.Success(fmt.Sprintf("Switched active tunnel to '%s'", tag))
	return nil
}
