package actions

func init() {
	Register(&Action{
		ID:    ActionUninstall,
		Use:   "uninstall",
		Short: "Completely uninstall dnstc",
		Long: `Remove all dnstc components from the system.

This will:
  - Stop all running tunnels
  - Remove all tunnel configurations
  - Remove downloaded binaries (slipstream-client, dnstt-client, sslocal)
  - Remove configuration files
  - Remove data files

Note: The dnstc binary itself is kept for easy reinstallation.`,
		MenuLabel: "Uninstall",
		Confirm: &ConfirmConfig{
			Message:     "Are you sure you want to uninstall everything?",
			Description: "This will remove all dnstc components from your system.",
			DefaultNo:   true,
			ForceFlag:   "force",
		},
	})
}
