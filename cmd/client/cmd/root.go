package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "omnitun",
		Short: "OmniTun Agent CLI",
		Long: `OmniTun Agent connects your internal services to the OmniTun platform,
enabling secure remote access through WebSocket and QUIC tunnels.

Manage tunnels, expose local services, and control your network
from the command line.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().String("api-url", "", "OmniTun API base URL (default: from session or https://api.omnitun.io)")
	rootCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")

	rootCmd.AddCommand(NewLoginCmd())
	rootCmd.AddCommand(NewLogoutCmd())
	rootCmd.AddCommand(NewHTTPCmd())
	rootCmd.AddCommand(NewTCPCmd())
	rootCmd.AddCommand(NewTunnelCmd())
	rootCmd.AddCommand(NewStatusCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewCompletionCmd())
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewLogsCmd())
	rootCmd.AddCommand(NewNetworkCmd())

	return rootCmd
}

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate autocompletion script for your shell.

Supported shells:
  bash        Generate Bash completion
  zsh         Generate Zsh completion
  fish        Generate Fish completion
  powershell  Generate PowerShell completion`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE:      runCompletion,
	}
	return cmd
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]
	root := cmd.Root()
	switch shell {
	case "bash":
		return root.GenBashCompletion(cmd.OutOrStdout())
	case "zsh":
		return root.GenZshCompletion(cmd.OutOrStdout())
	case "fish":
		return root.GenFishCompletion(cmd.OutOrStdout(), true)
	case "powershell":
		return root.GenPowerShellCompletion(cmd.OutOrStdout())
	default:
		return root.Help()
	}
}
