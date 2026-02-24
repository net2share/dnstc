package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
	"github.com/net2share/dnstc/internal/process"
)

const (
	uninstallServiceName = "dnstc"
	uninstallUnitPath    = "/etc/systemd/system/dnstc.service"
)

func init() {
	actions.SetHandler(actions.ActionUninstall, HandleUninstall)
}

// HandleUninstall performs a full system uninstall.
func HandleUninstall(ctx *actions.Context) error {
	beginProgress(ctx, "Uninstall dnstc")

	totalSteps := 6
	currentStep := 0

	// Step 1: Stop daemon if running
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Stopping daemon...")
	if running, client := ipc.DetectDaemon(); running {
		client.Stop()
		client.Shutdown()
		client.Close()
		ctx.Output.Status("Daemon stopped")
	} else if eng := engine.Get(); eng != nil {
		eng.Stop()
		ctx.Output.Status("Engine stopped")
	} else {
		ctx.Output.Status("No daemon running")
	}

	// Step 2: Stop orphan processes
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Cleaning up orphan processes...")
	mgr := process.NewManager(config.StatePath())
	mgr.StopAll()
	ctx.Output.Status("Orphan processes cleaned")

	// Step 3: Remove systemd unit if installed
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing systemd service...")
	if runtime.GOOS == "linux" {
		if _, err := os.Stat(uninstallUnitPath); err == nil {
			exec.Command("systemctl", "stop", uninstallServiceName).Run()
			exec.Command("systemctl", "disable", uninstallServiceName).Run()
			os.Remove(uninstallUnitPath)
			exec.Command("systemctl", "daemon-reload").Run()
			ctx.Output.Status("Systemd service removed")
		} else {
			ctx.Output.Status("No systemd service installed")
		}
	} else {
		ctx.Output.Status("Skipped (not Linux)")
	}

	// Step 4: Remove downloaded binaries
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing downloaded binaries...")
	binMgr := binaries.NewManager()
	defs := binaries.Defs()
	for _, name := range binaries.AllNames() {
		binMgr.Remove(defs[name])
	}
	os.Remove(config.BinDir())
	ctx.Output.Status("Binaries removed")

	// Step 5: Remove configuration directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing configuration...")
	os.RemoveAll(config.ConfigDir())
	ctx.Output.Status("Configuration removed")

	// Step 6: Remove data directory
	currentStep++
	ctx.Output.Step(currentStep, totalSteps, "Removing data files...")
	os.RemoveAll(filepath.Dir(config.BinDir()))
	ctx.Output.Status("Data files removed")

	ctx.Output.Success("Uninstallation complete!")
	ctx.Output.Info("The dnstc binary is still available for reinstallation.")

	endProgress(ctx)
	return nil
}
