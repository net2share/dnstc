package handlers

import (
	"github.com/net2share/dnstc/internal/engine"
	"github.com/net2share/dnstc/internal/ipc"
)

// NotifyDaemonReload tells a running daemon to reload its config.
// Best-effort: if no daemon is running, the saved config will be picked up on next start.
func NotifyDaemonReload() {
	if engine.Get() != nil {
		return // in-process engine, config already in memory
	}
	if running, client := ipc.DetectDaemon(); running {
		client.ReloadConfig()
		client.Close()
	}
}
