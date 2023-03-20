package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var completionInstallCommands = map[string]string{
	"bash": `This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

### Linux
$ ${name} completion bash --i
$ ${name} completion bash > /etc/bash_completion.d/${name}

### macOS
$ ${name} completion bash > $(brew --prefix)/etc/bash_completion.d/${name}`,
	"zsh": `Enable bash completion in Zsh:
% echo "autoload -U compinit; compinit" >> ~/.zshrc

### Linux:
% ${name} completion zsh > "${fpath[1]}/_${name}"
### macOS:
% ${name} completion zsh > $(brew --prefix)/share/zsh/site-functions/_${name}`,
	"fish": `Run the following command to enable fish completion:
> ${name} completion fish > ~/.config/fish/completions/${name}.fish`,
	"powershell": `Run the following command to enable powershell completion:
> ${name} completion powershell | Out-String | Invoke-Expression`,
}

func NewCompletionCmd(parent *cobra.Command) {
	parent.InitDefaultCompletionCmd()
	var cmd *cobra.Command
	for _, child := range parent.Commands() {
		if child.Name() == "completion" {
			cmd = child
			break
		}
	}
	if cmd == nil {
		return
	}
	noDesc := parent.CompletionOptions.DisableDescriptions

	for _, child := range cmd.Commands() {
		child.PreRunE = func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("print", cmd.Flags().Lookup("print")); err != nil {
				return err
			}
			return viper.BindPFlag("install", cmd.Flags().Lookup("install"))
		}
		child.RunE = func(cmd *cobra.Command, args []string) error {
			if viper.GetBool("print") {
				return cmd.Root().GenBashCompletionV2(parent.OutOrStdout(), !noDesc)
			}
			if filename := viper.GetString("install"); filename != "" {
				return cmd.Root().GenBashCompletionFileV2(filename, !noDesc)
			}
			command := completionInstallCommands[cmd.Name()]

			cmd.Println(strings.ReplaceAll(command, "${name}", cmd.Root().Name()))
			return nil
		}
	}
	cmd.PersistentFlags().BoolP("print", "p", false, "Prints the completion script to stdout")
	cmd.PersistentFlags().StringP("install", "i", "", "Installs the completion script to the specified location")
}
