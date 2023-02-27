package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewPushCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push a document to the gobin server",
		Long: `Push a document to the gobin server from std in. For example:

			gobin push
		
			Will push the document to the gobin server.
		
			You can also push a specific file. For example:
		
			gobin push -f /path/to/file
		
			Will push the file to the gobin server.
		
			You can also update a specific document. For example:
		
			gobin push -d jis74978
		
			Will update the document with the key of jis74978.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("push called with args: %v\n", args)

			file := viper.GetString("file")
			document := viper.GetString("document")
			server := viper.GetString("server")

			cmd.Printf("file: %s, document: %s, server: %s\n", file, document, server)

			var (
				r   io.Reader
				err error
			)

			if file != "" {
				r, err = os.Open(file)
				if err != nil {
					cmd.PrintErrln(err)
					return
				}
			} else {
				info, err := os.Stdin.Stat()
				if err != nil {
					cmd.PrintErrln(err)
					return
				}

				if info.Mode()&os.ModeNamedPipe == 0 {
					cmd.PrintErrln("no data from stdin")
					return
				}
				r = os.Stdin
			}

			content, err := io.ReadAll(r)
			if err != nil {
				cmd.PrintErr("failed to read from stdin", err)
				return
			}

			cmd.Println("content:", string(content))
		},
	}

	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "https://xgob.in", "Gobin server address")
	cmd.Flags().StringP("file", "f", "", "The file to push")
	cmd.Flags().StringP("document", "d", "", "The document to update")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("file", cmd.Flags().Lookup("file"))
	viper.BindPFlag("document", cmd.Flags().Lookup("document"))
}
