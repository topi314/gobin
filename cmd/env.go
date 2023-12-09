package cmd

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/topi314/gobin/internal/cfg"
)

func NewEnvCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Prints or sets gobin variables",
		Example: `gobin env

Will print all 

gobin env -w NAME=VALUE -w NAME2=VALUE2 -d NAME3

Will set NAME to VALUE in the gobin env (defaults to ~/.gobin).`,
		Args: cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			entries, err := cfg.Get()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			var names []string
			for name := range entries {
				names = append(names, name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("write", cmd.Flags().Lookup("write")); err != nil {
				return err
			}
			return viper.BindPFlag("delete", cmd.Flags().Lookup("delete"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			write := viper.GetStringSlice("write")
			del := viper.GetStringSlice("delete")

			if len(write) == 0 && len(del) == 0 {
				entries, err := cfg.Get()
				if err != nil {
					return fmt.Errorf("failed to get config: %w", err)
				}

				for name, value := range entries {
					if len(args) > 0 && !slices.Contains(args, name) {
						continue
					}
					cmd.Printf("%s='%s'\n", name, value)
				}
				return nil
			}

			if len(args) > 0 {
				return errors.New("invalid argument with -w flag")
			}

			_, err := cfg.Update(func(m map[string]string) {
				for _, kv := range write {
					kvs := strings.SplitN(kv, "=", 2)
					m[kvs[0]] = kvs[1]
				}
				for _, k := range del {
					delete(m, k)
				}
			})
			return err
		},
	}

	parent.AddCommand(cmd)
	cmd.Flags().StringSliceP("write", "w", nil, "Write one or more gobin variables")
	cmd.Flags().StringSliceP("delete", "d", nil, "Delete one or more gobin variables")

	if err := cmd.RegisterFlagCompletionFunc("write", configCompletion); err != nil {
		log.Printf("failed to register write flag completion func: %s", err)
	}

	if err := cmd.RegisterFlagCompletionFunc("delete", configCompletion); err != nil {
		log.Printf("failed to register delete flag completion func: %s", err)
	}
}

func configCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	entries, err := cfg.Get()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for name := range entries {
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
