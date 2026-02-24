package actions

// Action IDs for type-safe references throughout the codebase.
const (
	// Tunnel actions
	ActionTunnel         = "tunnel"
	ActionTunnelList     = "tunnel.list"
	ActionTunnelAdd      = "tunnel.add"
	ActionTunnelRemove   = "tunnel.remove"
	ActionTunnelStatus   = "tunnel.status"
	ActionTunnelActivate = "tunnel.activate"

	// Config actions
	ActionConfig            = "config"
	ActionConfigShow        = "config.show"
	ActionConfigEdit        = "config.edit"
	ActionConfigGatewayPort = "config.gateway-port"

	// System actions
	ActionInstall   = "install"
	ActionUpdate    = "update"
	ActionUninstall = "uninstall"
)
