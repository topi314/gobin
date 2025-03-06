package main

import (
	"github.com/topi314/gobin/v3/cli/cmd"
	"github.com/topi314/gobin/v3/internal/ver"
)

func main() {
	version := ver.Load()

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
