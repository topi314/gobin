package cmd

import (
	"fmt"
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
		Use:     "push",
		GroupID: "actions",
		Short:   "Pushes a document to the gobin server",
		Example: `gobin push "hello world!
		
Will push "hello world!" to the server`,
		Args: cobra.RangeArgs(0, 1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("server", cmd.Flags().Lookup("server")); err != nil {
				return err
			}
			if err := viper.BindPFlag("file", cmd.Flags().Lookup("file")); err != nil {
				return err
			}
			if err := viper.BindPFlag("document", cmd.Flags().Lookup("document")); err != nil {
				return err
			}
			if err := viper.BindPFlag("token", cmd.Flags().Lookup("token")); err != nil {
				return err
			}
			return viper.BindPFlag("language", cmd.Flags().Lookup("language"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file := viper.GetString("file")
			documentID := viper.GetString("document")
			token := viper.GetString("token")
			language := viper.GetString("language")

			var (
				r   io.Reader
				err error
			)
			if file != "" {
				r, err = os.Open(file)
				if err != nil {
					return fmt.Errorf("failed to open document file: %w", err)
				}
			} else {
				info, err := os.Stdin.Stat()
				if err != nil {
					return fmt.Errorf("failed to get stdin info: %w", err)
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
					return fmt.Errorf("no document provided")
				}
				content = args[0]
			} else {
				data, err := io.ReadAll(r)
				if err != nil {
					return fmt.Errorf("failed to read from std in or file: %w", err)
				}
				content = string(data)
			}

			contentReader := strings.NewReader(content)
			var rs *http.Response
			if documentID == "" {
				path := "/documents"
				if language != "" {
					path += "?language=" + language
				}
				rs, err = ezhttp.Post(path, contentReader)
				if err != nil {
					return fmt.Errorf("failed to create document: %w", err)
				}
			} else {
				if token == "" {
					token = viper.GetString("tokens_" + documentID)
				}
				if token == "" {
					return fmt.Errorf("no token found or provided for document: %s", documentID)
				}
				path := "/documents/" + documentID
				if language != "" {
					path += "?language=" + language
				}
				rs, err = ezhttp.Patch(path, token, contentReader)
				if err != nil {
					return fmt.Errorf("failed to update document: %w", err)
				}
			}
			defer rs.Body.Close()

			var documentRs gobin.DocumentResponse
			if err = ezhttp.ProcessBody("push document", rs, &documentRs); err != nil {
				return fmt.Errorf("failed to process response: %w", err)
			}

			method := "Updated"
			if documentID == "" {
				method = "Created"
			}
			cmd.Printf("%s document with ID: %s, Version: %d, URL: %s/%s\n", method, documentRs.Key, documentRs.Version, viper.GetString("server"), documentRs.Key)

			if documentID != "" {
				return nil
			}

			path, err := cfg.Update(func(m map[string]string) {
				m["TOKENS_"+documentRs.Key] = documentRs.Token
			})
			if err != nil {
				return fmt.Errorf("failed to update config: %w", err)
			}
			cmd.Println("Saved token to:", path)
			return nil
		},
	}

	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "", "Gobin server address")
	cmd.Flags().StringP("file", "f", "", "The file to push")
	cmd.Flags().StringP("document", "d", "", "The document to update")
	cmd.Flags().StringP("token", "t", "", "The token for the document to update")
	cmd.Flags().StringP("language", "l", "", "The language of the document")
}
