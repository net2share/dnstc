package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
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

	if oldAddr == newAddr {
		ctx.Output.Info(fmt.Sprintf("Gateway port unchanged (%d)", portVal))
		return nil
	}

	cfg.Listen.SOCKS = newAddr
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if oldAddr != "" {
		ctx.Output.Success(fmt.Sprintf("Gateway port changed: %s → %s", oldAddr, newAddr))
	} else {
		ctx.Output.Success(fmt.Sprintf("Gateway port set to %d", portVal))
	}

	// If engine is running, restart to apply the new port.
	if eng := engine.Get(); eng != nil && eng.IsConnected() {
		ctx.Output.Info("Restarting gateway...")
		eng.Stop()
		eng.ReloadConfig()
		if err := eng.Start(); err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}
		ctx.Output.Success("Gateway restarted on new port")
	} else if running, client := ipc.DetectDaemon(); running {
		ctx.Output.Info("Restarting daemon...")
		client.Stop()
		client.ReloadConfig()
		client.Start()
		client.Close()
		ctx.Output.Success("Daemon restarted on new port")
	}

	return nil
}
