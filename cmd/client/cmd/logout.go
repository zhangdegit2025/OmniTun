package cmd

import (
	"fmt"

	"github.com/omnitun/omnitun/internal/control"
	"github.com/spf13/cobra"
)

func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from the OmniTun control plane",
		Long:  "Remove locally stored credentials and invalidate the current session.",
		RunE:  runLogout,
	}
	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)

	session, err := control.LoadSession()
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		fmt.Println(dim("Not logged in."))
		return nil
	}

	client := control.NewAPIClient(apiURL, session.AccessToken)
	if err := client.Logout(); err != nil {
		fmt.Printf("%s Server logout failed: %v (clearing local session anyway)\n", yellow("!"), err)
	}

	if err := control.ClearSession(); err != nil {
		return fmt.Errorf("failed to clear session: %w", err)
	}

	fmt.Printf("%s Logged out successfully\n", green("✓"))
	return nil
}
