package main

import (
	"time"

	"github.com/topi314/gobin/cmd"
	"github.com/topi314/gobin/gobin"
)

// These variables are set via the -ldflags option in go build
var (
	Version   = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	cmd.NewGetCmd(rootCmd)
	cmd.NewPushCmd(rootCmd)
	cmd.NewRmCmd(rootCmd)
	buildTime, _ := time.Parse(time.RFC3339, BuildTime)
	cmd.NewVersionCmd(rootCmd, gobin.FormatBuildVersion(Version, Commit, buildTime))
	cmd.NewCompletionCmd(rootCmd)
	cmd.Execute(rootCmd)
}
