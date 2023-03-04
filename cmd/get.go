package cmd

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"os"
)

func NewGetCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get returns a document from the gobin server",
		Long: `Get returns a document from the gobin server. For example:

gobin get jis74978

Will return the document with the id of jis74978.

You can also save the document to a file. For example:

gobin get -f /path/to/file jis74978

Will save the document with the id of jis74978 to the file.

You can also get a specific version of a document. For example:

gobin get -v 123456 jis74978

Will return the document with the id of jis74978 and the version of 123456.`,
		Run: func(cmd *cobra.Command, args []string) {
			requestUrl := viper.GetString("server")
			document := args[0]

			if viper.GetString("version") != "" {
				requestUrl += "/raw/" + document + "/versions/" + viper.GetString("version")
			} else {
				requestUrl += "/raw/" + document
			}

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
				err := json.Unmarshal(body, &errorResponse)
				if err != nil {
					return
				}
				cmd.PrintErrln(viper.GetString("server") + " has returned a error: " + errorResponse.Message)
				return
			}

			if viper.GetString("file") != "" {
				file, err := os.Create(viper.GetString("file"))
				if err != nil {
					cmd.PrintErrln(err)
					return
				}
				defer file.Close()

				_, err = file.WriteString(content)
				if err != nil {
					cmd.PrintErrln(err)
					return
				}
			} else {
				cmd.Println(content)
			}

			//cmd.Println("get called with args:", args)
			//cmd.Println("server:", viper.GetString("server"))
		},
	}
	parent.AddCommand(cmd)

	cmd.Flags().StringP("server", "s", "https://xgob.in", "Gobin server address")
	cmd.Flags().StringP("file", "f", "", "The file to save the document to")
	cmd.Flags().StringP("version", "v", "", "The version of the document to get")

	viper.BindPFlag("server", cmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("file", cmd.Flags().Lookup("file"))
	viper.BindPFlag("version", cmd.Flags().Lookup("version"))
}
