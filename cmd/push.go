package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"os"
	"strings"
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
					r = nil
				} else {
					r = os.Stdin
				}
			}

			var content string
			if r == nil && len(args) > 0 {
				content = args[0]
			} else {
				bytes, err := io.ReadAll(r)
				content = string(bytes)
				if err != nil {
					cmd.PrintErr("failed to read from stdin", err)
					return
				}
			}

			client := &http.Client{}

			var (
				requestUrl string
				method     string
			)
			if document != "" {
				requestUrl = server + "/documents/" + document
				method = http.MethodPatch
			} else {
				requestUrl = server + "/documents"
				method = http.MethodPost
			}

			request, err := http.NewRequest(method, requestUrl, strings.NewReader(content))
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			response, err := client.Do(request)
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			defer response.Body.Close()
			responseContent, err := io.ReadAll(response.Body)
			if err != nil {
				cmd.PrintErr("failed to read from stdin", err)
				return
			}

			if response.StatusCode != http.StatusOK {
				cmd.PrintErrln(server + " returned status code: " + response.Status)
				return
			}

			body := string(responseContent)
			cmd.Printf("body: %s", body)
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
