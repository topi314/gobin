package main

import (
	"github.com/topisenpai/gobin/cmd"
)

// These variables are set via the -ldflags option in go build
var (
	version   = "unknown"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	cmd.NewGetCmd(rootCmd)
	cmd.NewPushCmd(rootCmd)
	cmd.NewVersionsCmd(rootCmd)
	cmd.NewVersionCmd(rootCmd, version, commit, buildTime)
	cmd.Execute(rootCmd)
}
