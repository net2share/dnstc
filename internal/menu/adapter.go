package menu

import (
	"context"
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/handlers"
	"github.com/net2share/go-corelib/tui"
)

// isInfoViewAction returns true for actions that manage their own TUI display
// (dialogs, editors, etc.) and should NOT be wrapped in a progress view.
func isInfoViewAction(actionID string) bool {
	switch actionID {
	case actions.ActionTunnelStatus:
		return true
	case actions.ActionTunnelAdd, actions.ActionTunnelRemove,
		actions.ActionUninstall, actions.ActionInstall, actions.ActionUpdate:
		return true
	case actions.ActionConfigEdit:
		return true
	}
	return false
}

// newActionContext creates a new action context.
func newActionContext(args []string) *actions.Context {
	ctx := &actions.Context{
		Ctx:           context.Background(),
		Args:          args,
		Values:        make(map[string]interface{}),
		Output:        handlers.NewTUIOutput(),
		IsInteractive: true,
	}

	cfg, _ := config.Load()
	ctx.Config = cfg

	return ctx
}

// BuildMenuOptions builds menu options from child actions.
func BuildMenuOptions(parentID string) []tui.MenuOption {
	var options []tui.MenuOption

	var cfg *config.Config
	cfg, _ = config.Load()

	children := actions.GetChildren(parentID)
	for _, action := range children {
		if action.ShowInMenu != nil {
			ctx := &actions.Context{Config: cfg}
			if !action.ShowInMenu(ctx) {
				continue
			}
		}

		if action.Hidden {
			continue
		}

		label := action.MenuLabel
		if label == "" {
			label = action.Short
		}

		if action.IsSubmenu {
			label += " →"
		}

		options = append(options, tui.MenuOption{
			Label: label,
			Value: action.ID,
		})
	}

	return options
}

// RunAction executes an action in interactive mode.
func RunAction(actionID string) error {
	action := actions.Get(actionID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", actionID)
	}

	ctx := &actions.Context{
		Ctx:           context.Background(),
		Values:        make(map[string]interface{}),
		Output:        handlers.NewTUIOutput(),
		IsInteractive: true,
	}

	cfg, _ := config.Load()
	ctx.Config = cfg

	// Handle argument collection
	if action.Args != nil {
		if action.Args.PickerFunc != nil {
			selected, err := runPickerForAction(ctx, action)
			if err != nil {
				if err == actions.ErrCancelled {
					return errCancelled
				}
				return err
			}
			if selected == "" {
				return errCancelled
			}
			ctx.Values[action.Args.Name] = selected
		} else if action.Args.Required {
			value, confirmed, err := tui.RunInput(tui.InputConfig{
				Title:       action.Args.Name,
				Description: action.Args.Description,
			})
			if err != nil {
				return err
			}
			if !confirmed || value == "" {
				return errCancelled
			}
			ctx.Values[action.Args.Name] = value
		}
	}

	// Collect inputs interactively
	for _, input := range action.Inputs {
		if input.ShowIf != nil && !input.ShowIf(ctx) {
			continue
		}

		var value interface{}

		switch input.Type {
		case actions.InputTypeText, actions.InputTypePassword:
			defaultVal := input.Default
			if input.DefaultFunc != nil {
				defaultVal = input.DefaultFunc(ctx)
			}

			description := input.Description
			if input.DescriptionFunc != nil {
				description = input.DescriptionFunc(ctx)
			}
			if defaultVal != "" && description != "" {
				description = fmt.Sprintf("%s (default: %s)", description, defaultVal)
			} else if defaultVal != "" {
				description = fmt.Sprintf("Default: %s", defaultVal)
			}

			var validationErr error
			for {
				desc := description
				if validationErr != nil {
					desc = fmt.Sprintf("%s\n⚠ %s", desc, validationErr.Error())
				}

				val, confirmed, inputErr := tui.RunInput(tui.InputConfig{
					Title:       input.Label,
					Description: desc,
					Placeholder: input.Placeholder,
					Value:       defaultVal,
					Password:    input.Type == actions.InputTypePassword,
				})
				if inputErr != nil {
					return inputErr
				}
				if !confirmed {
					return errCancelled
				}
				if val == "" && defaultVal != "" {
					val = defaultVal
				}
				if input.Required && val == "" {
					validationErr = fmt.Errorf("%s is required", input.Label)
					continue
				}
				if val != "" {
					if input.ValidateWithContext != nil {
						validationErr = input.ValidateWithContext(ctx, val)
					} else if input.Validate != nil {
						validationErr = input.Validate(val)
					}
					if validationErr != nil {
						continue
					}
				}
				value = val
				break
			}

		case actions.InputTypeNumber:
			defaultVal := input.Default
			if input.DefaultFunc != nil {
				defaultVal = input.DefaultFunc(ctx)
			}

			var validationErr error
			for {
				desc := input.Description
				if validationErr != nil {
					desc = fmt.Sprintf("%s\n⚠ %s", desc, validationErr.Error())
				}

				val, confirmed, inputErr := tui.RunInput(tui.InputConfig{
					Title:       input.Label,
					Description: desc,
					Value:       defaultVal,
				})
				if inputErr != nil {
					return inputErr
				}
				if !confirmed {
					return errCancelled
				}
				if val == "" && defaultVal != "" {
					val = defaultVal
				}
				if val != "" {
					if input.ValidateWithContext != nil {
						validationErr = input.ValidateWithContext(ctx, val)
					} else if input.Validate != nil {
						validationErr = input.Validate(val)
					}
					if validationErr != nil {
						defaultVal = val
						continue
					}
				}
				var intVal int
				fmt.Sscanf(val, "%d", &intVal)
				value = intVal
				break
			}

		case actions.InputTypeSelect:
			var tuiOptions []tui.MenuOption
			options := input.Options
			if input.OptionsFunc != nil {
				options = input.OptionsFunc(ctx)
			}
			for _, opt := range options {
				label := opt.Label
				if opt.Recommended {
					label += " (Recommended)"
				}
				tuiOptions = append(tuiOptions, tui.MenuOption{
					Label: label,
					Value: opt.Value,
				})
			}
			if !input.Required {
				tuiOptions = append(tuiOptions, tui.MenuOption{Label: "Skip", Value: ""})
			}

			selectDescription := input.Description
			if input.DescriptionFunc != nil {
				selectDescription = input.DescriptionFunc(ctx)
			}

			val, inputErr := tui.RunMenu(tui.MenuConfig{
				Title:       input.Label,
				Description: selectDescription,
				Options:     tuiOptions,
			})
			if inputErr != nil {
				return inputErr
			}
			if val == "" {
				if input.Required {
					return errCancelled
				}
				continue
			}
			value = val

		case actions.InputTypeBool:
			continue
		}

		ctx.Values[input.Name] = value
	}

	// Handle confirmation
	if action.Confirm != nil {
		confirm, err := tui.RunConfirm(tui.ConfirmConfig{
			Title:       action.Confirm.Message,
			Description: action.Confirm.Description,
			Default:     !action.Confirm.DefaultNo,
		})
		if err != nil {
			return err
		}
		if !confirm {
			return errCancelled
		}
	}

	if action.Handler == nil {
		return fmt.Errorf("no handler for action %s", action.ID)
	}

	tuiOut := ctx.Output.(*handlers.TUIOutput)
	if !isInfoViewAction(actionID) {
		tuiOut.BeginProgress(action.Short)
		defer tuiOut.EndProgress()
	}

	return action.Handler(ctx)
}

// runPickerForAction shows a picker for an action's argument.
func runPickerForAction(ctx *actions.Context, action *actions.Action) (string, error) {
	_, err := action.Args.PickerFunc(ctx)
	if err != nil {
		return "", err
	}

	options := actions.GetPickerOptions(ctx)
	if len(options) == 0 {
		return "", actions.NoTunnelsError()
	}

	var tuiOptions []tui.MenuOption
	for _, opt := range options {
		tuiOptions = append(tuiOptions, tui.MenuOption{
			Label: opt.Label,
			Value: opt.Value,
		})
	}
	tuiOptions = append(tuiOptions, tui.MenuOption{Label: "Back", Value: ""})

	selected, err := tui.RunMenu(tui.MenuConfig{
		Title:   fmt.Sprintf("Select %s", action.Args.Name),
		Options: tuiOptions,
	})
	if err != nil {
		return "", err
	}

	return selected, nil
}

// runActionWithArgs runs an action with predefined arguments, handling confirmation.
func runActionWithArgs(actionID string, args []string) error {
	action := actions.Get(actionID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", actionID)
	}

	// Handle confirmation with tag in message
	if action.Confirm != nil && len(args) > 0 {
		tag := args[0]
		confirm, err := tui.RunConfirm(tui.ConfirmConfig{
			Title:       fmt.Sprintf("%s '%s'?", action.Confirm.Message, tag),
			Description: action.Confirm.Description,
			Default:     !action.Confirm.DefaultNo,
		})
		if err != nil {
			return err
		}
		if !confirm {
			return errCancelled
		}
	}

	ctx := newActionContext(nil)
	if action.Args != nil && action.Args.Name == "tag" && len(args) > 0 {
		ctx.Values["tag"] = args[0]
	}

	if action.Handler == nil {
		return fmt.Errorf("no handler for action %s", actionID)
	}

	tuiOut := ctx.Output.(*handlers.TUIOutput)
	if !isInfoViewAction(actionID) {
		tuiOut.BeginProgress(action.Short)
		defer tuiOut.EndProgress()
	}

	return action.Handler(ctx)
}

// RunSubmenu runs a submenu loop for a parent action.
func RunSubmenu(parentID string) error {
	action := actions.Get(parentID)
	if action == nil {
		return fmt.Errorf("unknown action: %s", parentID)
	}

	for {
		options := BuildMenuOptions(parentID)
		options = append(options, tui.MenuOption{Label: "Back", Value: "back"})

		title := action.MenuLabel
		if title == "" {
			title = action.Short
		}

		choice, err := tui.RunMenu(tui.MenuConfig{
			Title:   title,
			Options: options,
		})
		if err != nil || choice == "" || choice == "back" {
			return errCancelled
		}

		childAction := actions.Get(choice)
		if childAction != nil && childAction.IsSubmenu {
			_ = RunSubmenu(choice)
			continue
		}

		if err := RunAction(choice); err != nil && err != errCancelled {
			_ = tui.ShowMessage(tui.AppMessage{Type: "error", Message: err.Error()})
		}
	}
}
