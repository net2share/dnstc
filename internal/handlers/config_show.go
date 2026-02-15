package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
)

func init() {
	actions.SetHandler(actions.ActionConfigShow, HandleConfigShow)
}

// HandleConfigShow shows the current configuration.
func HandleConfigShow(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		ctx.Output.Warning("No configuration found")
		ctx.Output.Info(fmt.Sprintf("Config path: %s", config.Path()))
		return nil
	}

	lines := []string{
		fmt.Sprintf("Config file: %s", config.Path()),
		"",
		fmt.Sprintf("SOCKS listen: %s", cfg.Listen.SOCKS),
	}

	if len(cfg.Resolvers) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Resolvers:")
		for _, r := range cfg.Resolvers {
			lines = append(lines, fmt.Sprintf("  - %s", r))
		}
	}

	if cfg.Route.Active != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Active tunnel: %s", cfg.Route.Active))
	}

	if len(cfg.Tunnels) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Tunnels: %d configured", len(cfg.Tunnels)))
		for _, tc := range cfg.Tunnels {
			activeMarker := ""
			if tc.Tag == cfg.Route.Active {
				activeMarker = " [active]"
			}
			lines = append(lines, fmt.Sprintf("  %s%s: %s/%s (%s)",
				tc.Tag, activeMarker,
				config.GetTransportTypeDisplayName(tc.Transport),
				config.GetBackendTypeDisplayName(tc.Backend),
				tc.Domain))
		}
	}

	ctx.Output.Box("Configuration", lines)
	return nil
}
