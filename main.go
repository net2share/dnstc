package main

import "github.com/net2share/dnstc/cmd"

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	cmd.SetVersionInfo(Version, BuildTime)
	cmd.Execute()
}
