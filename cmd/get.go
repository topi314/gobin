package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/topi314/gobin/gobin"
	"github.com/topi314/gobin/internal/ezhttp"
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("server", cmd.Flags().Lookup("server")); err != nil {
				return err
			}
			if err := viper.BindPFlag("file", cmd.Flags().Lookup("file")); err != nil {
				return err
			}
			if err := viper.BindPFlag("version", cmd.Flags().Lookup("version")); err != nil {
				return err
			}
			if err := viper.BindPFlag("versions", cmd.Flags().Lookup("versions")); err != nil {
				return err
			}
			if err := viper.BindPFlag("formatter", cmd.Flags().Lookup("formatter")); err != nil {
				return err
			}
			if err := viper.BindPFlag("language", cmd.Flags().Lookup("language")); err != nil {
				return err
			}
			return viper.BindPFlag("style", cmd.Flags().Lookup("style"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("document id is required")
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
					return fmt.Errorf("failed to get document versions: %w", err)
				}
				defer rs.Body.Close()

				var documentVersionsRs []gobin.DocumentResponse
				if err = ezhttp.ProcessBody("get document versions", rs, &documentVersionsRs); err != nil {
					return err
				}

				var documentVersions string
				for _, documentVersion := range documentVersionsRs {
					documentVersions += fmt.Sprintf("%d: %s\n", documentVersion.Version, humanize.Time(time.Unix(documentVersion.Version, 0)))
				}

				cmd.Printf("Document versions(%d):\n%s", len(documentVersions), documentVersions)
				return nil
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
				return fmt.Errorf("failed to get document: %w", err)
			}
			defer rs.Body.Close()

			var documentRs gobin.DocumentResponse
			if err = ezhttp.ProcessBody("get document", rs, &documentRs); err != nil {
				return err
			}

			data := documentRs.Data
			if formatter != "" {
				data = string(documentRs.Formatted)
			}

			if file == "" {
				cmd.Println(data)
				return nil
			}
			documentFile, err := os.Create(file)
			if err != nil {
				return fmt.Errorf("failed to create file to write document: %w", err)
			}
			defer documentFile.Close()

			_, err = documentFile.WriteString(data)
			if err != nil {
				return fmt.Errorf("failed to write document to file: %w", err)
			}
			cmd.Println("Document saved to file:", file)
			return nil
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
