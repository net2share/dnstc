package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/go-corelib/binman"
)

func init() {
	actions.SetHandler(actions.ActionInstall, HandleInstall)
}

// HandleInstall downloads and installs all required binaries.
func HandleInstall(ctx *actions.Context) error {
	beginProgress(ctx, "Install Binaries")

	mgr := binaries.NewManager()
	defs := binaries.Defs()
	names := binaries.AllNames()
	total := len(names)

	manifest := binman.NewManifest()

	for i, name := range names {
		def := defs[name]
		step := i + 1

		if !mgr.IsPlatformSupported(def) {
			ctx.Output.Step(step, total, fmt.Sprintf("Skipping %s (unsupported platform)", name))
			continue
		}

		if mgr.IsInstalled(def) {
			ctx.Output.Step(step, total, fmt.Sprintf("%s already installed", name))
			manifest.SetVersion(name, def.PinnedVersion)
			continue
		}

		ctx.Output.Step(step, total, fmt.Sprintf("Downloading %s...", name))

		if err := mgr.Download(def, def.PinnedVersion, nil); err != nil {
			ctx.Output.Error(fmt.Sprintf("Failed to install %s: %v", name, err))
			continue
		}

		manifest.SetVersion(name, def.PinnedVersion)
		ctx.Output.Status(fmt.Sprintf("%s installed", name))
	}

	if err := manifest.Save(config.VersionsPath()); err != nil {
		ctx.Output.Warning(fmt.Sprintf("Failed to save version manifest: %v", err))
	}

	ctx.Output.Success("Binary installation complete")

	endProgress(ctx)
	return nil
}
