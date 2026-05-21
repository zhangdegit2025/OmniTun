package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/omnitun/omnitun/internal/control"
	"github.com/spf13/cobra"
)

func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the OmniTun control plane",
		Long:  "Authenticate with your OmniTun account to create and manage tunnels.",
		RunE:  runLogin,
	}
	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)

	fmt.Print("Email: ")
	reader := bufio.NewReader(os.Stdin)
	email, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	password, err := readPassword()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println()

	if email == "" || password == "" {
		return fmt.Errorf("email and password are required")
	}

	fmt.Printf("\n%s Authenticating...\n", dim("→"))

	client := control.NewAPIClient(apiURL, "")
	resp, err := client.Login(email, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	session := &control.SessionFile{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		APIBaseURL:   apiURL,
		ExpiresAt:    time.Now().Unix() + resp.ExpiresIn,
		Email:        resp.Email,
	}

	if err := control.SaveSession(session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	fmt.Printf("\n%s Logged in as %s\n", green("✓"), bold(resp.Email))
	return nil
}
