package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewGetCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get returns a document from the gobin server",
		Long: `Get returns a document from the gobin server. For example:

gobin get jis74978

Will return the document with the id of jis74978.

You can also save the document to a file. For example:

gobin get -f /path/to/file jis74978

Will save the document with the id of jis74978 to the file.

You can also get a specific version of a document. For example:

gobin get -v 123456 jis74978

Will return the document with the id of jis74978 and the version of 123456.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("get called with args:", args)
			cmd.Println("server:", viper.GetString("server"))
		},
	}
	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "https://xgob.in", "Gobin server address")
	cmd.Flags().StringP("file", "f", "", "The file to save the document to")
	cmd.Flags().StringP("version", "v", "", "The version of the document to get")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("file", cmd.Flags().Lookup("file"))
	viper.BindPFlag("version", cmd.Flags().Lookup("version"))
}
