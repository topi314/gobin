package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/topisenpai/gobin/gobin"
	"github.com/topisenpai/gobin/internal/ezhttp"
)

func NewGetCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Gets a document from the gobin server",
		Long: `Gets a document from the gobin server. For example:

gobin get jis74978

Will return the document with the id of jis74978.

You can also save the document to a file. For example:

gobin get -f /path/to/file jis74978

Will save the document with the id of jis74978 to the file.

You can also get a specific version of a document. For example:

gobin get -v 123456 jis74978

Will return the document with the id of jis74978 and the version of 123456.`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.PrintErrln("document id is required")
				return
			}
			documentID := args[0]
			file := viper.GetString("file")
			version := viper.GetString("version")
			versions := viper.GetBool("versions")
			render := viper.GetString("render")

			if versions {
				url := "/documents/" + documentID + "/versions"
				rs, err := ezhttp.Get(url)
				if err != nil {
					cmd.PrintErrln("Failed to get document versions:", err)
					return
				}
				defer rs.Body.Close()

				var documentVersionsRs []gobin.DocumentResponse
				if ok := ezhttp.ProcessBody(cmd, "get document versions", rs, &documentVersionsRs); !ok {
					return
				}

				now := time.Now()
				var documentVersions string
				for _, documentVersion := range documentVersionsRs {
					relative, _ := gobin.FormatDocumentVersion(now, documentVersion.Version)
					documentVersions += fmt.Sprintf("%d: %s\n", documentVersion.Version, relative)
				}

				cmd.Printf("Document versions(%d):\n%s", len(documentVersions), documentVersions)
				return
			}

			url := "/documents/" + documentID
			if version != "" {
				url += "/versions/" + version
			}
			if render != "" {
				url += "?render=" + render
			}

			rs, err := ezhttp.Get(url)
			if err != nil {
				cmd.PrintErrln("Failed to get document:", err)
				return
			}
			defer rs.Body.Close()

			var documentRs gobin.DocumentResponse
			if ok := ezhttp.ProcessBody(cmd, "get document", rs, &documentRs); !ok {
				return
			}

			data := documentRs.Data
			if render != "" {
				data = string(documentRs.Formatted)
			}

			if file == "" {
				cmd.Println(data)
				return
			}
			documentFile, err := os.Create(file)
			if err != nil {
				cmd.PrintErrln("Failed to create file to write document:", err)
				return
			}
			defer documentFile.Close()

			_, err = documentFile.WriteString(data)
			if err != nil {
				cmd.PrintErrln("Failed to write document to file:", err)
				return
			}
			cmd.Println("Document saved to file:", file)
		},
	}
	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "", "Gobin server address")
	cmd.Flags().StringP("file", "f", "", "The file to save the document to")
	cmd.Flags().StringP("version", "v", "", "The version of the document to get")
	cmd.Flags().BoolP("versions", "", false, "Get all versions of the document")
	cmd.Flags().StringP("render", "r", "", "Render the document with syntax highlighting (terminal8, terminal16, terminal256, terminal16m, html, or none)")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("file", cmd.Flags().Lookup("file"))
	viper.BindPFlag("version", cmd.Flags().Lookup("version"))
	viper.BindPFlag("versions", cmd.Flags().Lookup("versions"))
	viper.BindPFlag("render", cmd.Flags().Lookup("render"))
}
