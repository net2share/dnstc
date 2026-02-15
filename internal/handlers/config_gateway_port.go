package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
)

func init() {
	actions.SetHandler(actions.ActionConfigGatewayPort, HandleConfigGatewayPort)
}

// HandleConfigGatewayPort sets the gateway proxy port.
func HandleConfigGatewayPort(ctx *actions.Context) error {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		cfg = config.Default()
		ctx.Config = cfg
	}

	portVal := ctx.GetInt("port")
	if portVal == 0 {
		return fmt.Errorf("--port is required")
	}

	newAddr := fmt.Sprintf("127.0.0.1:%d", portVal)
	oldAddr := cfg.Listen.SOCKS

	cfg.Listen.SOCKS = newAddr
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if oldAddr != "" && oldAddr != newAddr {
		ctx.Output.Success(fmt.Sprintf("Gateway port changed: %s → %s", oldAddr, newAddr))
		ctx.Output.Info("Restart dnstc for the change to take effect.")
	} else {
		ctx.Output.Success(fmt.Sprintf("Gateway port set to %d", portVal))
	}

	return nil
}
