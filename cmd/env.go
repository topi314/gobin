package cmd

import (
	"errors"
	"fmt"
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

gobin env -w NAME=VALUE - w NAME2=VALUE2

Will set NAME to VALUE in the gobin env (defaults to ~/.gobin).`,
		Args: cobra.ArbitraryArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlag("write", cmd.Flags().Lookup("write"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			write := viper.GetStringSlice("write")

			if len(write) == 0 {
				entries, err := cfg.Get()
				if err != nil {
					return fmt.Errorf("failed to get config: %w", err)
				}

				for i := range args {
					args[i] = strings.ToUpper(args[i])
				}

				for name, value := range entries {
					if len(args) > 0 && !slices.Contains(args, strings.ToUpper(name)) {
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
					m[strings.ToUpper(kvs[0])] = kvs[1]
				}
			})
			return err
		},
	}

	parent.AddCommand(cmd)
	cmd.Flags().StringSliceP("write", "w", nil, "Write one or more gobin variables")

}
