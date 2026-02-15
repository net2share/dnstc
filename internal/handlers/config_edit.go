package handlers

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
)

func init() {
	actions.SetHandler(actions.ActionConfigEdit, HandleConfigEdit)
}

// HandleConfigEdit opens the configuration in an editor.
func HandleConfigEdit(ctx *actions.Context) error {
	configPath := config.Path()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	// Create default config if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.Default()
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
	}

	editorCmd := exec.Command(editor, configPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	return editorCmd.Run()
}
