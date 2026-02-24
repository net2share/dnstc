// Package handlers provides the business logic for dnstc actions.
package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
)

// LoadConfig loads and caches the configuration.
func LoadConfig(ctx *actions.Context) (*config.Config, error) {
	if ctx.Config != nil {
		return ctx.Config, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	ctx.Config = cfg
	return cfg, nil
}

// GetTunnelByTag retrieves a tunnel by tag from the config.
func GetTunnelByTag(ctx *actions.Context, tag string) (*config.TunnelConfig, error) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	tunnel := cfg.GetTunnelByTag(tag)
	if tunnel == nil {
		return nil, actions.TunnelNotFoundError(tag)
	}

	return tunnel, nil
}

// RequireTag gets the tag from context args, returning a standardized error if empty.
func RequireTag(ctx *actions.Context) (string, error) {
	tag := ctx.GetArg(0)
	if tag == "" {
		tag = ctx.GetString("tag")
	}
	if tag == "" {
		return "", actions.NewActionError("tunnel tag required", "Usage: dnstc tunnel <command> <tag>")
	}
	return tag, nil
}

// RequireTunnels returns an error if no tunnels are configured.
func RequireTunnels(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Tunnels) == 0 {
		return actions.NoTunnelsError()
	}

	return nil
}

// RequireEngine returns the running engine or an error if not running.
func RequireEngine() (engine.EngineController, error) {
	eng := engine.Get()
	if eng == nil {
		return nil, actions.NewActionError(
			"no engine running",
			"Start with: dnstc (interactive) or dnstc daemon start",
		)
	}
	return eng, nil
}

// beginProgress starts a progress view in interactive mode.
func beginProgress(ctx *actions.Context, title string) {
	if ctx.IsInteractive {
		ctx.Output.BeginProgress(title)
	}
}

// endProgress ends a progress view in interactive mode.
func endProgress(ctx *actions.Context) {
	if ctx.IsInteractive {
		ctx.Output.EndProgress()
	}
}

// failProgress shows an error in the progress view and returns the error.
func failProgress(ctx *actions.Context, err error) error {
	if ctx.IsInteractive {
		ctx.Output.Error(fmt.Sprintf("Failed: %v", err))
		ctx.Output.EndProgress()
	}
	return err
}
