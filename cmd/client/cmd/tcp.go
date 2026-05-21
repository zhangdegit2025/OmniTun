package cmd

import (
	"github.com/spf13/cobra"
)

func NewTCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tcp [local_host:port]",
		Short: "Expose a TCP service via OmniTun tunnel",
		Long: `Create and start a TCP tunnel that exposes a local TCP service to the internet.

Examples:
  omnitun tcp 3306
  omnitun tcp 127.0.0.1:5432`,
		Args: cobra.ExactArgs(1),
		RunE: runTCP,
	}
	return cmd
}

func runTCP(cmd *cobra.Command, args []string) error {
	return runHTTP(cmd, args)
}
