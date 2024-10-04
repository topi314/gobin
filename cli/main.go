package main

import (
	"time"

	cmd2 "github.com/topi314/gobin/v2/cli/cmd"
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

	rootCmd := cmd2.NewRootCmd()
	cmd2.NewGetCmd(rootCmd)
	cmd2.NewPostCmd(rootCmd)
	cmd2.NewRmCmd(rootCmd)
	cmd2.NewImportCmd(rootCmd)
	cmd2.NewShareCmd(rootCmd)
	cmd2.NewVersionCmd(rootCmd, ver.FormatBuildVersion(Version, Commit, buildTime))
	cmd2.NewEnvCmd(rootCmd)
	cmd2.NewCompletionCmd(rootCmd)
	cmd2.Execute(rootCmd)
}
