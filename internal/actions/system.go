package actions

func init() {
	Register(&Action{
		ID:        ActionInstall,
		Use:       "install",
		Short:     "Install required binaries",
		Long:      "Download and install all required transport binaries",
		MenuLabel: "Install Binaries",
	})

	Register(&Action{
		ID:              ActionUpdate,
		Use:             "update",
		Short:           "Check for and apply updates",
		Long:            "Check for updates to dnstc and transport binaries",
		MenuLabel:       "Check Updates",
		RequiresInstall: true,
		Inputs: []InputField{
			{
				Name:  "check",
				Label: "Check only (don't apply)",
				Type:  InputTypeBool,
			},
			{
				Name:  "self",
				Label: "Update dnstc only",
				Type:  InputTypeBool,
			},
			{
				Name:  "binaries",
				Label: "Update binaries only",
				Type:  InputTypeBool,
			},
		},
	})

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
