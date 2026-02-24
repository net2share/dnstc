package actions

import (
	"fmt"
	"strconv"

	"github.com/net2share/dnstc/internal/port"
)

func init() {
	// Config parent action (submenu)
	Register(&Action{
		ID:              ActionConfig,
		Use:             "config",
		Short:           "Manage configuration",
		Long:            "Show or edit configuration",
		MenuLabel:       "Configure",
		IsSubmenu:       true,
		RequiresInstall: true,
	})

	// config show
	Register(&Action{
		ID:        ActionConfigShow,
		Parent:    ActionConfig,
		Use:       "show",
		Short:     "Show current configuration",
		Long:      "Display the current configuration",
		MenuLabel: "Show",
	})

	// config edit
	Register(&Action{
		ID:        ActionConfigEdit,
		Parent:    ActionConfig,
		Use:       "edit",
		Short:     "Edit configuration",
		Long:      "Open configuration in editor",
		MenuLabel: "Edit",
	})

	// config gateway-port
	Register(&Action{
		ID:        ActionConfigGatewayPort,
		Parent:    ActionConfig,
		Use:       "gateway-port",
		Short:     "Set gateway proxy port",
		Long:      "Set the local SOCKS port for the gateway proxy",
		MenuLabel: "Gateway Port",
		Inputs: []InputField{
			{
				Name:        "port",
				Label:       "Gateway Port",
				ShortFlag:   'p',
				Type:        InputTypeNumber,
				Required:    true,
				Description: "Local SOCKS port for the gateway proxy",
				DefaultFunc: func(ctx *Context) string {
					if ctx.Config != nil && ctx.Config.Listen.SOCKS != "" {
						_, portStr, err := parseHostPort(ctx.Config.Listen.SOCKS)
						if err == nil {
							return portStr
						}
					}
					return "1080"
				},
				ValidateWithContext: func(ctx *Context, value string) error {
					p, err := strconv.Atoi(value)
					if err != nil || p <= 0 || p > 65535 {
						return fmt.Errorf("invalid port number")
					}
					// Skip port-in-use check if it matches current config
					// (the gateway itself may be listening on it).
					if ctx.Config != nil && ctx.Config.Listen.SOCKS != "" {
						_, currentPort, err := parseHostPort(ctx.Config.Listen.SOCKS)
						if err == nil && currentPort == value {
							return nil
						}
					}
					if !port.IsAvailable(p) {
						return fmt.Errorf("port %d is already in use", p)
					}
					return nil
				},
			},
		},
	})
}

func parseHostPort(addr string) (string, string, error) {
	host, portStr, err := splitHostPort(addr)
	return host, portStr, err
}

func splitHostPort(addr string) (string, string, error) {
	// Simple host:port split
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "", fmt.Errorf("no port in address")
}
