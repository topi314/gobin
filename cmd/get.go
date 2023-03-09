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
		Use:     "get",
		GroupID: "actions",
		Short:   "Gets a document from the gobin server",
		Example: `gobin get jis74978

Will return the document with the id of jis74978.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			tokensMap := viper.GetStringMap("tokens.")
			tokens := make([]string, 0, len(tokensMap))
			for document := range tokensMap {
				tokens = append(tokens, document)
			}
			return tokens, cobra.ShellCompDirectiveNoFileComp
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
			viper.BindPFlag("file", cmd.Flags().Lookup("file"))
			viper.BindPFlag("version", cmd.Flags().Lookup("version"))
			viper.BindPFlag("versions", cmd.Flags().Lookup("versions"))
			viper.BindPFlag("formatter", cmd.Flags().Lookup("formatter"))
			viper.BindPFlag("language", cmd.Flags().Lookup("language"))
			viper.BindPFlag("style", cmd.Flags().Lookup("style"))
		},
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.PrintErrln("document id is required")
				return
			}
			documentID := args[0]
			file := viper.GetString("file")
			version := viper.GetString("version")
			versions := viper.GetBool("versions")
			formatter := viper.GetString("formatter")
			language := viper.GetString("language")
			style := viper.GetString("style")

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
			if formatter != "" {
				url += "?formatter=" + formatter
				if language != "" {
					url += "&language=" + language
				}
				if style != "" {
					url += "&style=" + style
				}
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
			if formatter != "" {
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
	cmd.Flags().StringP("formatter", "r", "", "Format the document with syntax highlighting (terminal8, terminal16, terminal256, terminal16m, html, html-standalone, svg, or none)")
	cmd.Flags().StringP("language", "l", "", "The language to render the document with")
	cmd.Flags().StringP("style", "", "", "The style to render the document with")
}
