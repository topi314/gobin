package cmd

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/topi314/gobin/gobin"
	"github.com/topi314/gobin/internal/cfg"
	"github.com/topi314/gobin/internal/ezhttp"
)

func NewPostCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "post",
		GroupID: "actions",
		Short:   "Posts a document to the gobin server",
		Example: `gobin post "hello world!"
		
Will post "hello world!" to the server`,
		Args: cobra.ArbitraryArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("server", cmd.Flags().Lookup("server")); err != nil {
				return err
			}
			if err := viper.BindPFlag("files", cmd.Flags().Lookup("files")); err != nil {
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
			files := viper.GetStringSlice("files")
			documentID := viper.GetString("document")
			token := viper.GetString("token")
			language := viper.GetString("language")

			var (
				readers []io.Reader
			)
			if len(files) > 0 {
				for _, file := range files {
					fr, err := os.Open(strings.TrimSpace(file))
					if err != nil {
						return fmt.Errorf("failed to open document file: %w", err)
					}
					readers = append(readers, fr)
				}
			} else {
				info, err := os.Stdin.Stat()
				if err != nil {
					return fmt.Errorf("failed to get stdin info: %w", err)
				}

				if info.Mode()&os.ModeNamedPipe != 0 {
					readers = append(readers, os.Stdin)
				}
			}
			defer func() {
				for _, r := range readers {
					if rc, ok := r.(io.Closer); ok {
						_ = rc.Close()
					}
				}
			}()

			if len(readers) == 0 {
				if len(args) == 0 {
					return fmt.Errorf("no document provided")
				}
				if len(args) == 1 {
					readers = append(readers, bytes.NewReader([]byte(args[0])))
				} else {
					for _, arg := range args {
						readers = append(readers, bytes.NewReader([]byte(arg)))
					}
				}
			}

			var r io.Reader
			if len(readers) == 0 {
				if file, ok := r.(*os.File); ok {
					r = ezhttp.NewHeaderReader(file, http.Header{
						"Content-Type": []string{
							mime.FormatMediaType("application/octet-stream", map[string]string{
								"filename": file.Name(),
							}),
						},
					})
				} else {
					r = readers[0]
				}
			} else {
				buff := new(bytes.Buffer)
				mpw := multipart.NewWriter(buff)

				for i, rr := range readers {
					fileName := fmt.Sprintf("untitiled%d", i)
					if file, ok := rr.(*os.File); ok {
						fileName = file.Name()
					}
					part, err := mpw.CreatePart(textproto.MIMEHeader{
						"Content-Disposition": []string{
							mime.FormatMediaType("form-data", map[string]string{
								"name":     fmt.Sprintf("file-%d", i),
								"filename": fileName,
							}),
						},
					})
					if err != nil {
						return fmt.Errorf("failed to create multipart part")
					}
					if _, err = io.Copy(part, rr); err != nil {
						return fmt.Errorf("failed to write multipart part")
					}
				}

				if err := mpw.Close(); err != nil {
					return fmt.Errorf("failed to close multipart writer")
				}
				r = ezhttp.NewHeaderReader(buff, http.Header{
					"Content-Type": []string{
						mpw.FormDataContentType(),
					},
				})
			}

			var (
				rs  *http.Response
				err error
			)
			if documentID == "" {
				path := "/documents"
				if language != "" {
					path += "?language=" + language
				}
				rs, err = ezhttp.Post(path, r)
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
				rs, err = ezhttp.Patch(path, token, r)
				if err != nil {
					return fmt.Errorf("failed to update document: %w", err)
				}
			}
			defer rs.Body.Close()

			var documentRs gobin.DocumentResponse
			if err = ezhttp.ProcessBody("post document", rs, &documentRs); err != nil {
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
	cmd.Flags().StringSliceP("files", "f", nil, "The files to post")
	cmd.Flags().StringP("document", "d", "", "The document to update")
	cmd.Flags().StringP("token", "t", "", "The token for the document to update")
	cmd.Flags().StringP("language", "l", "", "The language of the document")
}
