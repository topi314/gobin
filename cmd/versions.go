package cmd

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"net/http"
)

func NewVersionsCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Get returns all versions of a document from the gobin server",
		Long: `Get returns all versions of a document from the gobin server. For example:

gobin versions jis74978

Will return all versions of the document with the id of jis74978.`,
		Run: func(cmd *cobra.Command, args []string) {
			requestUrl := viper.GetString("server")
			document := args[0]

			requestUrl += "/documents/" + document + "/versions"
			cmd.Printf("requestUrl: %s", requestUrl)

			client := &http.Client{}

			response, err := client.Get(requestUrl)
			defer response.Body.Close()

			if err != nil {
				cmd.PrintErrln(err)
				return
			}

			body, err := io.ReadAll(response.Body)
			if err != nil {
				cmd.PrintErrln(err)
				return
			}

			content := string(body)

			if response.StatusCode != http.StatusOK {
				var errorResponse ErrorResponse
				err = json.Unmarshal(body, &errorResponse)
				if err != nil {
					cmd.PrintErrln(err)
					return
				}

				cmd.PrintErrln(viper.GetString("server") + " responded with an error: " + errorResponse.Message)
				return
			}

			cmd.Println(content)
		},
	}
	parent.AddCommand(cmd) ///

	cmd.Flags().StringP("server", "s", "https://xgob.in", "Gobin server address")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
}
