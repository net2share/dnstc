package actions

import (
	"fmt"
	"strconv"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/port"
)

func init() {
	// Tunnel parent action (submenu)
	Register(&Action{
		ID:        ActionTunnel,
		Use:       "tunnel",
		Short:     "Manage tunnels",
		Long:      "Manage DNS tunnel configurations",
		MenuLabel: "Tunnels",
		IsSubmenu: true,
	})

	// tunnel list
	Register(&Action{
		ID:        ActionTunnelList,
		Parent:    ActionTunnel,
		Use:       "list",
		Short:     "List all tunnels",
		Long:      "List all configured DNS tunnels and their status",
		MenuLabel: "List",
	})

	// tunnel status
	Register(&Action{
		ID:        ActionTunnelStatus,
		Parent:    ActionTunnel,
		Use:       "status",
		Short:     "Show tunnel status",
		Long:      "Show status and configuration for a tunnel",
		MenuLabel: "Status",
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// tunnel start
	Register(&Action{
		ID:        ActionTunnelStart,
		Parent:    ActionTunnel,
		Use:       "start",
		Short:     "Start a tunnel",
		Long:      "Start a DNS tunnel",
		MenuLabel: "Start",
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// tunnel stop
	Register(&Action{
		ID:        ActionTunnelStop,
		Parent:    ActionTunnel,
		Use:       "stop",
		Short:     "Stop a tunnel",
		Long:      "Stop a running DNS tunnel",
		MenuLabel: "Stop",
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// tunnel restart
	Register(&Action{
		ID:        ActionTunnelRestart,
		Parent:    ActionTunnel,
		Use:       "restart",
		Short:     "Restart a tunnel",
		Long:      "Restart a DNS tunnel",
		MenuLabel: "Restart",
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// tunnel remove
	Register(&Action{
		ID:        ActionTunnelRemove,
		Parent:    ActionTunnel,
		Use:       "remove",
		Short:     "Remove a tunnel",
		Long:      "Remove a tunnel and its configuration",
		MenuLabel: "Remove",
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
		Confirm: &ConfirmConfig{
			Message:   "Remove tunnel?",
			DefaultNo: true,
			ForceFlag: "force",
		},
	})

	// tunnel activate
	Register(&Action{
		ID:        ActionTunnelActivate,
		Parent:    ActionTunnel,
		Use:       "activate",
		Short:     "Set active tunnel",
		Long:      "Set a tunnel as the active route",
		MenuLabel: "Activate",
		Args: &ArgsSpec{
			Name:        "tag",
			Description: "Tunnel tag",
			Required:    true,
			PickerFunc:  TunnelPicker,
		},
	})

	// tunnel add
	Register(&Action{
		ID:        ActionTunnelAdd,
		Parent:    ActionTunnel,
		Use:       "add",
		Short:     "Add a new tunnel",
		Long:      "Add a new DNS tunnel interactively or via flags",
		MenuLabel: "Add",
		Inputs: []InputField{
			{
				Name:        "tag",
				Label:       "Tag",
				ShortFlag:   't',
				Type:        InputTypeText,
				Description: "Tunnel tag (auto-generated if omitted)",
				ShowIf:      func(ctx *Context) bool { return !ctx.IsInteractive },
			},
			{
				Name:        "transport",
				Label:       "Transport",
				Type:        InputTypeSelect,
				Required:    true,
				Options:     TransportOptions(),
				Description: "The transport protocol to use",
			},
			{
				Name:        "backend",
				Label:       "Backend",
				ShortFlag:   'b',
				Type:        InputTypeSelect,
				Required:    true,
				OptionsFunc: BackendOptionsForTransport,
				Description: "The backend type",
			},
			{
				Name:        "domain",
				Label:       "Domain",
				ShortFlag:   'd',
				Type:        InputTypeText,
				Required:    true,
				Placeholder: "t1.example.com",
				Description: "DNS tunnel domain",
			},
			{
				Name:      "port",
				Label:     "Local Port",
				ShortFlag: 'p',
				Type:      InputTypeNumber,
				Description: "Local SOCKS port",
				DefaultFunc: func(ctx *Context) string {
					p, err := port.GetAvailable()
					if err != nil {
						return "1080"
					}
					return strconv.Itoa(p)
				},
				Validate: func(value string) error {
					p, err := strconv.Atoi(value)
					if err != nil || p <= 0 || p > 65535 {
						return fmt.Errorf("invalid port number")
					}
					if !port.IsAvailable(p) {
						return fmt.Errorf("port %d is already in use", p)
					}
					return nil
				},
			},
			{
				Name:        "pubkey",
				Label:       "Public Key",
				Type:        InputTypeText,
				Description: "DNSTT public key (64 hex characters)",
				Validate:    ValidatePubkey,
				ShowIf: func(ctx *Context) bool {
					return config.TransportType(ctx.GetString("transport")) == config.TransportDNSTT
				},
			},
			{
				Name:        "cert",
				Label:       "Certificate Path",
				Type:        InputTypeText,
				Description: "Slipstream certificate path (optional)",
				ShowIf: func(ctx *Context) bool {
					return config.TransportType(ctx.GetString("transport")) == config.TransportSlipstream &&
						config.BackendType(ctx.GetString("backend")) == config.BackendSOCKS
				},
			},
			{
				Name:        "ss-server",
				Label:       "Shadowsocks Server",
				Type:        InputTypeText,
				Description: "Shadowsocks server address (host:port)",
				ShowIf: func(ctx *Context) bool {
					return config.BackendType(ctx.GetString("backend")) == config.BackendShadowsocks
				},
			},
			{
				Name:        "ss-password",
				Label:       "Shadowsocks Password",
				Type:        InputTypePassword,
				Description: "Shadowsocks password",
				ShowIf: func(ctx *Context) bool {
					return config.BackendType(ctx.GetString("backend")) == config.BackendShadowsocks
				},
			},
			{
				Name:        "ss-method",
				Label:       "Encryption Method",
				Type:        InputTypeSelect,
				Options:     EncryptionMethodOptions(),
				Default:     "chacha20-ietf-poly1305",
				Description: "Shadowsocks encryption method",
				ShowIf: func(ctx *Context) bool {
					return config.BackendType(ctx.GetString("backend")) == config.BackendShadowsocks
				},
			},
		},
	})
}
