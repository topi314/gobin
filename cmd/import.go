package cmd

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/topi314/gobin/v2/internal/cfg"
)

func NewImportCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "import",
		GroupID: "actions",
		Short:   "Imports a token from a share link",
		Example: `gobin import https://xgob.in/jis74978?token=kiczgez33j7qkvqdg9f7ksrd8jk88wba

Will import the token for the document jis74978 and server https://xgob.in`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("server", cmd.Flags().Lookup("server")); err != nil {
				return err
			}
			return viper.BindPFlag("document", cmd.Flags().Lookup("document"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("document link/token is required")
			}
			var (
				server     string
				documentID string
				token      string
			)
			if uri, err := url.Parse(args[0]); err == nil {
				server = uri.Scheme + "://" + uri.Host
				documentID = strings.SplitN(uri.Path, "/", 2)[0]
				token = uri.Query().Get("token")
			} else {
				token = args[0]
				documentID = viper.GetString("document")
				server = viper.GetString("server")
			}
			if server == "" {
				return fmt.Errorf("server is required")
			}
			if documentID == "" {
				return fmt.Errorf("document id is required")
			}

			path, err := cfg.Update(func(m map[string]string) {
				m["TOKENS_"+documentID] = token
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
	cmd.Flags().StringP("document", "d", "", "The document id to import the token for")

	if err := cmd.RegisterFlagCompletionFunc("document", documentCompletion); err != nil {
		log.Printf("failed to register document flag completion func: %s", err)
	}
}
