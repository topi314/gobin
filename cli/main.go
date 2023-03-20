package main

import (
	"github.com/topisenpai/gobin/cmd"
	"github.com/topisenpai/gobin/gobin"
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
	cmd.NewRmCmd(rootCmd)
	cmd.NewVersionCmd(rootCmd, gobin.FormatBuildVersion(version, commit, buildTime))
	cmd.NewCompletionCmd(rootCmd)
	cmd.Execute(rootCmd)
}
