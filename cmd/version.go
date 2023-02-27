package cmd

import (
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

func NewVersionCmd(parent *cobra.Command, version string, commit string, buildTime string) {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Returns the version of the gobin cli",
		Long: `Returns the version of the gobin cli. For example:

gobin version

Go Version: go1.18.3
Version: dev
Commit: b1fd421
Build Time: Mon Jan  1 00:00:00 0001`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(commit) > 7 {
				commit = commit[:7]
			}

			buildTimeStr := "unknown"
			if buildTime != "unknown" {
				parsedTime, _ := time.Parse(time.RFC3339, buildTime)
				if !parsedTime.IsZero() {
					buildTimeStr = parsedTime.Format(time.ANSIC)
				}
			}

			cmd.Printf("Go Version: %s\nVersion: %s\nCommit: %s\nBuild Time: %s\nOS/Arch: %s/%s\n", runtime.Version(), version, commit, buildTimeStr, runtime.GOOS, runtime.GOARCH)
		},
	}

	parent.AddCommand(cmd)
}
