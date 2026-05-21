package cmd

import (
	"fmt"
	"os"

	"github.com/omnitun/omnitun/internal/control"
	"github.com/spf13/cobra"
)

func getAPIURL(cmd *cobra.Command) string {
	if flagURL, _ := cmd.Flags().GetString("api-url"); flagURL != "" {
		return flagURL
	}
	root := cmd.Root()
	if root != nil {
		if flagURL, _ := root.PersistentFlags().GetString("api-url"); flagURL != "" {
			return flagURL
		}
	}
	return control.DefaultAPIURL()
}

func isVerbose(cmd *cobra.Command) bool {
	root := cmd.Root()
	if root != nil {
		v, _ := root.PersistentFlags().GetBool("verbose")
		return v
	}
	return false
}

func isJSON(cmd *cobra.Command) bool {
	root := cmd.Root()
	if root != nil {
		j, _ := root.PersistentFlags().GetBool("json")
		return j
	}
	return false
}

func loadSessionAndClient(apiURL string) (*control.SessionFile, *control.APIClient, error) {
	session, err := control.LoadSession()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		return nil, nil, fmt.Errorf("not logged in. Run '%s login' first", os.Args[0])
	}
	client := control.NewAPIClient(apiURL, session.AccessToken)
	return session, client, nil
}
