package main

import (
	"time"

	"github.com/topi314/gobin/v2/cli/cmd"
	"github.com/topi314/gobin/v2/internal/ver"
)

// These variables are set via the -ldflags option in go build
var (
	Version   = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	buildTime, _ := time.Parse(time.RFC3339, BuildTime)
	version := ver.FormatBuildVersion(Version, Commit, buildTime)

	rootCmd := cmd.NewRootCmd()
	cmd.NewGetCmd(rootCmd)
	cmd.NewPostCmd(rootCmd)
	cmd.NewRmCmd(rootCmd)
	cmd.NewImportCmd(rootCmd)
	cmd.NewShareCmd(rootCmd)
	cmd.NewVersionCmd(rootCmd, version)
	cmd.NewEnvCmd(rootCmd)
	cmd.NewCompletionCmd(rootCmd)
	cmd.Execute(rootCmd)
}
