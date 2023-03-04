package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/topisenpai/gobin/gobin"
	"github.com/topisenpai/gobin/internal/cfg"
	"github.com/topisenpai/gobin/internal/ezhttp"
)

func NewRmCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a document from the gobin server",
		Long: `Remove a document from the gobin server. For example:

gobin rm jis74978

Will remove the document to the gobin server.

You can also rm a specific version. For example:

gobin push -v 1 jis74978

Will remove the version to the gobin server.`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.PrintErrln("document id is required")
				return
			}
			documentID := args[0]
			version := viper.GetString("version")
			token := viper.GetString("token")

			path := "/documents/" + documentID
			if version != "" {
				path += "/versions/" + version
			}

			if token == "" {
				token = viper.GetString("tokens_" + documentID)
			}
			if token == "" {
				cmd.PrintErrln("No token found or provided for document:", documentID)
				return
			}

			rs, err := ezhttp.Delete(path, token)
			if err != nil {
				cmd.PrintErrln("Failed to create document:", err)
				return
			}
			defer rs.Body.Close()

			if rs.StatusCode != 200 && rs.StatusCode != 204 {
				var errRs gobin.ErrorResponse
				if err = json.NewDecoder(rs.Body).Decode(&errRs); err != nil {
					cmd.PrintErrln("Failed to decode error response:", err)
					return
				}
				cmd.PrintErrln("Failed to remove document:", errRs.Message)
				return
			}

			var deleteRs gobin.DeleteResponse
			if version != "" {
				if err = json.NewDecoder(rs.Body).Decode(&deleteRs); err != nil {
					cmd.PrintErrln("Failed to decode delete response:", err)
					return
				}
				cmd.Printf("Removed version: %s from document: %s\n", version, documentID)
			} else {
				cmd.Printf("Removed document: %s\n", documentID)

			}
			if deleteRs.Versions > 0 {
				return
			}

			path, err = cfg.Update(func(m map[string]string) {
				delete(m, "TOKENS_"+documentID)
			})
			if err != nil {
				cmd.PrintErrln("Failed to update config:", err)
				return
			}
			cmd.Printf("Removed document: %s from config: %s\n", documentID, path)
		},
	}

	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "", "Gobin server address")
	cmd.Flags().StringP("version", "v", "", "The version to update")
	cmd.Flags().StringP("token", "t", "", "The token for the document to update")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("version", cmd.Flags().Lookup("version"))
	viper.BindPFlag("token", cmd.Flags().Lookup("token"))
}
