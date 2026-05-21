package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Manage OmniTun tunnels",
		Long:  "List, start, stop, and delete tunnels.",
	}

	cmd.AddCommand(newTunnelListCmd())
	cmd.AddCommand(newTunnelStartCmd())
	cmd.AddCommand(newTunnelStopCmd())
	cmd.AddCommand(newTunnelDeleteCmd())

	return cmd
}

func newTunnelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tunnels",
		RunE:  runTunnelList,
	}
}

func newTunnelStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <tunnel_id>",
		Short: "Start a tunnel",
		Args:  cobra.ExactArgs(1),
		RunE:  runTunnelStart,
	}
}

func newTunnelStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <tunnel_id>",
		Short: "Stop a tunnel",
		Args:  cobra.ExactArgs(1),
		RunE:  runTunnelStop,
	}
}

func newTunnelDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <tunnel_id>",
		Short: "Delete a tunnel",
		Args:  cobra.ExactArgs(1),
		RunE:  runTunnelDelete,
	}
}

func runTunnelList(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	session, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}
	_ = session

	tunnels, err := client.ListTunnels()
	if err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}

	if len(tunnels) == 0 {
		fmt.Println(dim("No tunnels found."))
		return nil
	}

	fmt.Printf("\n%s  %-36s %-10s %-12s %s\n",
		dim("  "), dim("ID"), dim("STATUS"), dim("PROTOCOL"), dim("ADDRESS"))
	fmt.Println(dim("  " + strings.Repeat("─", 80)))

	for _, t := range tunnels {
		statusColor := green
		if t.Status != "active" {
			statusColor = dim
		}
		addr := fmt.Sprintf("%s:%d → :%d", t.LocalHost, t.LocalPort, t.RemotePort)
		fmt.Printf("  %-36s %-10s %-12s %s\n",
			dim(t.ID),
			statusColor(t.Status),
			dim(t.Protocol),
			dim(addr),
		)
	}
	fmt.Println()
	return nil
}

func runTunnelStart(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	tunnelID := args[0]
	fmt.Printf("%s Starting tunnel %s ...\n", dim("→"), bold(tunnelID[:8]))

	_, err = client.StartTunnel(context.Background(), tunnelID)
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	fmt.Printf("%s Tunnel %s started\n", green("✓"), bold(tunnelID[:8]))
	return nil
}

func runTunnelStop(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	tunnelID := args[0]
	fmt.Printf("%s Stopping tunnel %s ...\n", dim("→"), bold(tunnelID[:8]))

	if err := client.StopTunnel(tunnelID); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	fmt.Printf("%s Tunnel %s stopped\n", green("✓"), bold(tunnelID[:8]))
	return nil
}

func runTunnelDelete(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	tunnelID := args[0]

	fmt.Printf("%s Are you sure you want to delete tunnel %s? [y/N]: ", yellow("!"), bold(tunnelID[:8]))
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	fmt.Printf("%s Deleting tunnel %s ...\n", dim("→"), bold(tunnelID[:8]))

	if err := client.DeleteTunnel(tunnelID); err != nil {
		return fmt.Errorf("failed to delete tunnel: %w", err)
	}

	fmt.Printf("%s Tunnel %s deleted\n", green("✓"), bold(tunnelID[:8]))
	return nil
}
