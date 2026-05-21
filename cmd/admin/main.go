package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "omnitun-admin",
		Short: "OmniTun Admin CLI",
		Long:  "OmniTun Admin CLI provides management tools for the OmniTun SaaS platform.",
	}

	rootCmd.AddCommand(userCmd())
	rootCmd.AddCommand(tenantCmd())
	rootCmd.AddCommand(relayCmd())
	rootCmd.AddCommand(migrateCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create [email]",
		Short: "Create a new user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("user create not yet implemented")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("user list not yet implemented")
		},
	})

	return cmd
}

func tenantCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tenant",
		Short: "Manage tenants",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("tenant command not yet implemented")
		},
	}
}

func relayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "relay",
		Short: "Manage relay nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("relay command not yet implemented")
		},
	}
}

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("migrate command not yet implemented")
		},
	}
}
