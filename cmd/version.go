package cmd

import (
	"github.com/spf13/cobra"
)

func NewVersionCmd(parent *cobra.Command, version string) {
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
			cmd.Print(version)
		},
	}

	parent.AddCommand(cmd)
}
