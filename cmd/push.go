package cmd

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/topisenpai/gobin/gobin"
	"github.com/topisenpai/gobin/internal/cfg"
	"github.com/topisenpai/gobin/internal/ezhttp"
)

func NewPushCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Pushes a document to the gobin server",
		Long: `Pushes a document to the gobin server from std in. For example:

gobin push

Will push the document to the gobin server.

You can also push a specific file. For example:

gobin push -f /path/to/file

Will push the file to the gobin server.

You can also update a specific document. For example:

gobin push -d jis74978

Will update the document with the key of jis74978.`,
		Run: func(cmd *cobra.Command, args []string) {
			file := viper.GetString("file")
			documentID := viper.GetString("document")
			token := viper.GetString("token")

			var (
				r   io.Reader
				err error
			)
			if file != "" {
				r, err = os.Open(file)
				if err != nil {
					cmd.PrintErrln("Failed to open document file:", err)
					return
				}
			} else {
				info, err := os.Stdin.Stat()
				if err != nil {
					cmd.PrintErrln("Failed to get stdin info:", err)
					return
				}

				if info.Mode()&os.ModeNamedPipe == 0 {
					r = nil
				} else {
					r = os.Stdin
				}
			}

			var content string
			if r == nil {
				if len(args) == 0 {
					cmd.PrintErrln("no document provided")
					return
				}
				content = args[0]
			} else {
				data, err := io.ReadAll(r)
				if err != nil {
					cmd.PrintErrln("Failed to read from std in or file:", err)
					return
				}
				content = string(data)
			}

			contentReader := strings.NewReader(content)
			var rs *http.Response
			if documentID == "" {
				rs, err = ezhttp.Post("/documents", "", contentReader)
				if err != nil {
					cmd.PrintErrln("Failed to create document:", err)
					return
				}
			} else {
				if token == "" {
					token = viper.GetString("tokens_" + documentID)
				}
				if token == "" {
					cmd.PrintErrln("No token found or provided for document:", documentID)
					return
				}
				rs, err = ezhttp.Patch("/documents/"+documentID, token, "", contentReader)
				if err != nil {
					cmd.PrintErrln("Failed to update document:", err)
					return
				}
			}
			defer rs.Body.Close()

			var documentRs gobin.DocumentResponse
			if ok := ezhttp.ProcessBody(cmd, "push document", rs, &documentRs); !ok {
				return
			}

			method := "Updated"
			if documentID == "" {
				method = "Created"
			}
			cmd.Printf("%s document with ID: %s, Version: %d, URL: %s/%s\n", method, documentRs.Key, documentRs.Version, viper.GetString("server"), documentRs.Key)

			if documentID != "" {
				return
			}

			path, err := cfg.Update(func(m map[string]string) {
				m["TOKENS_"+documentRs.Key] = documentRs.Token
			})
			if err != nil {
				cmd.PrintErrln("Failed to update config:", err)
				return
			}
			cmd.Println("Saved token to:", path)
		},
	}

	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "", "Gobin server address")
	cmd.Flags().StringP("file", "f", "", "The file to push")
	cmd.Flags().StringP("document", "d", "", "The document to update")
	cmd.Flags().StringP("token", "t", "", "The token for the document to update")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("file", cmd.Flags().Lookup("file"))
	viper.BindPFlag("document", cmd.Flags().Lookup("document"))
	viper.BindPFlag("token", cmd.Flags().Lookup("token"))
}
