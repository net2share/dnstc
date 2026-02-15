package handlers

import (
	"os"
	"path/filepath"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/download"
	"github.com/net2share/dnstc/internal/engine"
)

func init() {
	actions.SetHandler(actions.ActionUninstall, HandleUninstall)
}

// HandleUninstall performs a full system uninstall.
func HandleUninstall(ctx *actions.Context) error {
	beginProgress(ctx, "Uninstall dnstc")

	totalSteps := 4
	currentStep := 0

	// Step 1: Stop all running tunnels via engine
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Stopping running tunnels...")
	if eng := engine.Get(); eng != nil {
		eng.Stop()
	}
	ctx.Output.Status("Tunnels stopped")

	// Step 2: Remove downloaded binaries
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing downloaded binaries...")
	binaries := []string{download.BinarySlipstream, download.BinaryDNSTT, download.BinaryShadowsocks}
	for _, b := range binaries {
		if download.IsBinaryInstalled(b) {
			download.RemoveBinary(b)
		}
	}
	os.Remove(config.BinDir())
	ctx.Output.Status("Binaries removed")

	// Step 3: Remove configuration directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing configuration...")
	os.RemoveAll(config.ConfigDir())
	ctx.Output.Status("Configuration removed")

	// Step 4: Remove data directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing data files...")
	os.RemoveAll(filepath.Dir(config.BinDir()))
	ctx.Output.Status("Data files removed")

	ctx.Output.Success("Uninstallation complete!")
	ctx.Output.Info("The dnstc binary is still available for reinstallation.")

	endProgress(ctx)
	return nil
}
