package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long:  "Generate the autocompletion script for momorph for the specified shell.",
	Example: `  # Bash (Linux)
  momorph completion bash > /etc/bash_completion.d/momorph

  # Bash (macOS with Homebrew)
  momorph completion bash > $(brew --prefix)/etc/bash_completion.d/momorph

  # Zsh (macOS with Homebrew)
  momorph completion zsh > $(brew --prefix)/share/zsh/site-functions/_momorph

  # Fish
  momorph completion fish > ~/.config/fish/completions/momorph.fish

  # PowerShell
  momorph completion powershell >> $PROFILE`,
	DisableFlagsInUseLine: true,
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate the autocompletion script for bash",
	Example: `  # Load in current session
  source <(momorph completion bash)

  # Linux - load permanently
  sudo momorph completion bash > /etc/bash_completion.d/momorph

  # macOS (Homebrew) - load permanently
  momorph completion bash > $(brew --prefix)/etc/bash_completion.d/momorph`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenBashCompletionV2(os.Stdout, true)
	},
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate the autocompletion script for zsh",
	Example: `  # Load in current session
  source <(momorph completion zsh)

  # Linux - load permanently
  momorph completion zsh > "${fpath[1]}/_momorph"

  # macOS (Homebrew) - load permanently
  momorph completion zsh > $(brew --prefix)/share/zsh/site-functions/_momorph`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenZshCompletion(os.Stdout)
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate the autocompletion script for fish",
	Example: `  # Load in current session
  momorph completion fish | source

  # Load permanently
  momorph completion fish > ~/.config/fish/completions/momorph.fish`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenFishCompletion(os.Stdout, true)
	},
}

var completionPowershellCmd = &cobra.Command{
	Use:   "powershell",
	Short: "Generate the autocompletion script for powershell",
	Example: `  # Load in current session
  momorph completion powershell | Out-String | Invoke-Expression

  # Load permanently (add to your PowerShell profile)
  momorph completion powershell >> $PROFILE`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
	},
}

func init() {
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)
	completionCmd.AddCommand(completionPowershellCmd)
	rootCmd.AddCommand(completionCmd)
}
