package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/handlers"
	"github.com/spf13/cobra"
)

// BuildCobraCommand builds a Cobra command from an action.
func BuildCobraCommand(action *actions.Action) *cobra.Command {
	cmd := &cobra.Command{
		Use:    action.Use,
		Short:  action.Short,
		Long:   action.Long,
		Hidden: action.Hidden,
	}

	// Add flags for inputs
	for _, input := range action.Inputs {
		if input.InteractiveOnly {
			continue
		}
		switch input.Type {
		case actions.InputTypeText, actions.InputTypePassword:
			if input.ShortFlag != 0 {
				cmd.Flags().StringP(input.Name, string(input.ShortFlag), input.Default, input.Label)
			} else {
				cmd.Flags().String(input.Name, input.Default, input.Label)
			}
		case actions.InputTypeNumber:
			defaultVal := 0
			if input.Default != "" {
				if v, err := strconv.Atoi(input.Default); err == nil {
					defaultVal = v
				}
			}
			if input.ShortFlag != 0 {
				cmd.Flags().IntP(input.Name, string(input.ShortFlag), defaultVal, input.Label)
			} else {
				cmd.Flags().Int(input.Name, defaultVal, input.Label)
			}
		case actions.InputTypeSelect:
			if input.ShortFlag != 0 {
				cmd.Flags().StringP(input.Name, string(input.ShortFlag), input.Default, input.Label)
			} else {
				cmd.Flags().String(input.Name, input.Default, input.Label)
			}
		case actions.InputTypeBool:
			cmd.Flags().Bool(input.Name, false, input.Label)
		}
	}

	// Register --tag/-t flag from Args when no Input already defines it
	if action.Args != nil && action.Args.Name == "tag" {
		hasTagInput := false
		for _, input := range action.Inputs {
			if input.Name == "tag" {
				hasTagInput = true
				break
			}
		}
		if !hasTagInput {
			cmd.Flags().StringP("tag", "t", "", action.Args.Description)
		}
	}

	// Handle confirmation flag
	if action.Confirm != nil && action.Confirm.ForceFlag != "" {
		cmd.Flags().BoolP(action.Confirm.ForceFlag, "f", false, "Skip confirmation")
	}

	// Submenus have no RunE but propagate install check to children
	if action.IsSubmenu {
		if action.RequiresInstall {
			cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
				return requireInstall()
			}
		}
		return cmd
	}

	// Set up the run function
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if action.RequiresInstall {
			if err := requireInstall(); err != nil {
				return err
			}
		}

		ctx := &actions.Context{
			Ctx:           context.Background(),
			Args:          args,
			Values:        make(map[string]interface{}),
			Output:        handlers.NewTUIOutput(),
			IsInteractive: false,
		}

		// Load config
		cfg, _ := config.Load()
		ctx.Config = cfg

		// Collect tag from --tag/-t flag
		if action.Args != nil && action.Args.Name == "tag" {
			tagVal, _ := cmd.Flags().GetString("tag")
			ctx.Values["tag"] = tagVal
			if action.Args.Required && tagVal == "" {
				return fmt.Errorf("--tag/-t is required\n\nUsage: %s", cmd.UseLine())
			}
		}

		// Collect values from flags
		for _, input := range action.Inputs {
			if input.InteractiveOnly {
				continue
			}
			switch input.Type {
			case actions.InputTypeText, actions.InputTypePassword, actions.InputTypeSelect:
				val, _ := cmd.Flags().GetString(input.Name)
				ctx.Values[input.Name] = val
			case actions.InputTypeNumber:
				val, _ := cmd.Flags().GetInt(input.Name)
				ctx.Values[input.Name] = val
			case actions.InputTypeBool:
				val, _ := cmd.Flags().GetBool(input.Name)
				ctx.Values[input.Name] = val
			}
		}

		// Handle confirmation flag
		if action.Confirm != nil && action.Confirm.ForceFlag != "" {
			force, _ := cmd.Flags().GetBool(action.Confirm.ForceFlag)
			ctx.Values[action.Confirm.ForceFlag] = force
		}

		// Require non-tag arguments in CLI mode
		if action.Args != nil && action.Args.Name != "tag" && action.Args.Required && len(args) == 0 {
			return fmt.Errorf("%s is required\n\nUsage: %s", action.Args.Name, cmd.UseLine())
		}

		// Handle confirmation — require --force in CLI mode
		if action.Confirm != nil {
			force := ctx.GetBool(action.Confirm.ForceFlag)
			if !force {
				return fmt.Errorf("%s\n\nUse --force to confirm", action.Confirm.Message)
			}
		}

		if action.Handler == nil {
			return fmt.Errorf("no handler for action %s", action.ID)
		}

		return action.Handler(ctx)
	}

	return cmd
}

// requireInstall checks that binaries are installed, returning a user-friendly error if not.
func requireInstall() error {
	if !binaries.AreInstalled() {
		return fmt.Errorf("binaries not installed — run 'dnstc install' first")
	}
	return nil
}

// RegisterActionsWithRoot adds all action-based commands to a root command.
func RegisterActionsWithRoot(root *cobra.Command) {
	for _, action := range actions.TopLevel() {
		cmd := BuildCobraCommand(action)
		for _, child := range actions.GetChildren(action.ID) {
			childCmd := BuildCobraCommand(child)
			cmd.AddCommand(childCmd)
		}
		root.AddCommand(cmd)
	}
}
