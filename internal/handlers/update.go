package handlers

import (
	"fmt"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/dnstc/internal/binaries"
	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/go-corelib/binman"
)

// AppVersion is set by cmd at startup for use by the update handler.
var AppVersion = "dev"

func init() {
	actions.SetHandler(actions.ActionUpdate, HandleUpdate)
}

// HandleUpdate checks for and applies updates.
func HandleUpdate(ctx *actions.Context) error {
	beginProgress(ctx, "Check Updates")

	checkOnly := ctx.GetBool("check")
	selfOnly := ctx.GetBool("self")
	binariesOnly := ctx.GetBool("binaries")

	currentVersion := AppVersion
	hasUpdates := false

	// Self-update check
	if !binariesOnly {
		ctx.Output.Status("Checking for dnstc updates...")

		latestVersion, available, err := binman.CheckSelfUpdate("net2share/dnstc", currentVersion)
		if err != nil {
			ctx.Output.Warning(fmt.Sprintf("Failed to check dnstc version: %v", err))
		} else if available {
			hasUpdates = true
			ctx.Output.Info(fmt.Sprintf("dnstc update available: %s → %s", currentVersion, latestVersion))

			if !checkOnly {
				err := binman.SelfUpdate(binman.SelfUpdateConfig{
					Repo:       "net2share/dnstc",
					URLPattern: "https://github.com/net2share/dnstc/releases/download/{version}/dnstc-{os}-{arch}",
					StatusFn: func(msg string) {
						ctx.Output.Status(msg)
					},
				}, latestVersion)
				if err != nil {
					ctx.Output.Error(fmt.Sprintf("Self-update failed: %v", err))
				} else {
					ctx.Output.Success(fmt.Sprintf("dnstc updated to %s", latestVersion))
				}
			}
		} else {
			ctx.Output.Status(fmt.Sprintf("dnstc is up to date (%s)", currentVersion))
		}
	}

	// Binary updates
	if !selfOnly {
		ctx.Output.Status("Checking binary updates...")

		manifest, err := binman.LoadManifest(config.VersionsPath())
		if err != nil {
			ctx.Output.Warning(fmt.Sprintf("Failed to load version manifest: %v", err))
			manifest = binman.NewManifest()
		}

		mgr := binaries.NewManager()
		defs := binaries.Defs()

		for _, name := range binaries.AllNames() {
			def := defs[name]
			if def.SkipUpdate {
				continue
			}

			currentVer := manifest.GetVersion(name)
			pinnedVer := def.PinnedVersion

			if binman.IsNewer(currentVer, pinnedVer) {
				hasUpdates = true
				ctx.Output.Info(fmt.Sprintf("%s: %s → %s", name, currentVer, pinnedVer))

				if !checkOnly {
					ctx.Output.Status(fmt.Sprintf("Updating %s...", name))
					if err := mgr.Download(def, pinnedVer, nil); err != nil {
						ctx.Output.Error(fmt.Sprintf("Failed to update %s: %v", name, err))
						continue
					}
					manifest.SetVersion(name, pinnedVer)
					ctx.Output.Success(fmt.Sprintf("%s updated to %s", name, pinnedVer))
				}
			} else {
				ctx.Output.Status(fmt.Sprintf("%s is up to date (%s)", name, currentVer))
			}
		}

		if !checkOnly {
			if err := manifest.Save(config.VersionsPath()); err != nil {
				ctx.Output.Warning(fmt.Sprintf("Failed to save version manifest: %v", err))
			}
		}
	}

	if !hasUpdates {
		ctx.Output.Success("Everything is up to date")
	} else if checkOnly {
		ctx.Output.Info("Run 'dnstc update' to apply updates")
	}

	endProgress(ctx)
	return nil
}
