package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewVersionsCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Get returns all versions of a document from the gobin server",
		Long: `Get returns all versions of a document from the gobin server. For example:

gobin versions jis74978

Will return all versions of the document with the id of jis74978.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("get called with args: %v", args)
		},
	}
	parent.AddCommand(cmd) ///

	cmd.Flags().StringP("server", "s", "https://xgob.in", "Gobin server address")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
}
