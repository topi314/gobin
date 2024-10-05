package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/topi314/gobin/v2/internal/cfg"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gobin",
		Short:        "gobin let's you upload and download documents from the gobin server",
		Long:         "",
		SilenceUsage: true,
	}
	cmd.AddGroup(&cobra.Group{
		ID:    "actions",
		Title: "Actions",
	})

	var cfgFile string
	cmd.PersistentFlags().StringVar(&cfgFile, "config", os.Getenv("GOBIN_CONFIG"), "config file (default is $HOME/.gobin)")
	cmd.PersistentFlags().BoolP("help", "h", false, "help for gobin")
	cmd.CompletionOptions.DisableDescriptions = true
	cobra.OnInitialize(initConfig(cfgFile))

	return cmd
}

func Execute(command *cobra.Command) {
	err := command.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initConfig(cfgFile string) func() {
	return func() {
		viper.SetDefault("server", "https://xgob.in")
		viper.SetDefault("formatter", "terminal16m")
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
		} else {
			home, err := os.UserHomeDir()
			cobra.CheckErr(err)

			viper.SetConfigName(".gobin")
			viper.SetConfigType("env")
			viper.AddConfigPath(home)
		}
		viper.SetEnvPrefix("gobin")
		viper.AutomaticEnv()

		_ = viper.ReadInConfig()
	}
}

func documentCompletion(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	entries, err := cfg.Get()
	if err != nil {
		cmd.Printf("failed to get config entries: %s\n", err)
		return nil, cobra.ShellCompDirectiveError
	}

	var documents []string
	for entry := range entries {
		if strings.HasPrefix(entry, "TOKENS_") {
			documents = append(documents, strings.TrimPrefix(entry, "TOKENS_"))
		}
	}
	return documents, cobra.ShellCompDirectiveNoFileComp
}
