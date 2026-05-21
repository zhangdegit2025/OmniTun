package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage mesh networks",
		Long:  "List, create, join, leave, and inspect mesh networks.",
	}
	cmd.AddCommand(newNetworkListCmd())
	cmd.AddCommand(newNetworkCreateCmd())
	cmd.AddCommand(newNetworkJoinCmd())
	cmd.AddCommand(newNetworkLeaveCmd())
	cmd.AddCommand(newNetworkStatusCmd())
	return cmd
}

func newNetworkListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List mesh networks",
		RunE:  runNetworkList,
	}
}

func newNetworkCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new mesh network",
		Args:  cobra.ExactArgs(1),
		RunE:  runNetworkCreate,
	}
}

func newNetworkJoinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "join <network>",
		Short: "Join a mesh network via name or invite code",
		Args:  cobra.ExactArgs(1),
		RunE:  runNetworkJoin,
	}
}

func newNetworkLeaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "leave <network>",
		Short: "Leave a mesh network",
		Args:  cobra.ExactArgs(1),
		RunE:  runNetworkLeave,
	}
}

func newNetworkStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <network>",
		Short: "Show network details and nodes",
		Args:  cobra.ExactArgs(1),
		RunE:  runNetworkStatus,
	}
}

func runNetworkList(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	networks, err := client.ListNetworks()
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	if len(networks) == 0 {
		fmt.Println(dim("No mesh networks found."))
		return nil
	}

	fmt.Printf("\n%s  %-36s %-20s %s\n",
		dim("  "), dim("ID"), dim("NAME"), dim("NODES"))
	fmt.Println(dim("  " + strings.Repeat("─", 70)))
	for _, n := range networks {
		fmt.Printf("  %-36s %-20s %s%d\n",
			dim(n.ID),
			bold(n.Name),
			dim("  "),
			n.NodeCount,
		)
	}
	fmt.Println()
	return nil
}

func runNetworkCreate(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	name := args[0]
	network, err := client.CreateNetwork(name)
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	fmt.Printf("%s Network created: %s (%s)\n", green("✓"), bold(network.Name), dim(network.ID))
	return nil
}

func runNetworkJoin(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	inviteOrName := args[0]
	network, err := client.JoinNetwork(inviteOrName)
	if err != nil {
		return fmt.Errorf("failed to join network: %w", err)
	}

	fmt.Printf("%s Joined network: %s (%d nodes)\n", green("✓"), bold(network.Name), network.NodeCount)
	return nil
}

func runNetworkLeave(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	networkID := args[0]
	if err := client.LeaveNetwork(networkID); err != nil {
		return fmt.Errorf("failed to leave network: %w", err)
	}

	fmt.Printf("%s Left network %s\n", green("✓"), dim(networkID))
	return nil
}

func runNetworkStatus(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	networkID := args[0]
	network, err := client.GetNetworkStatus(networkID)
	if err != nil {
		return fmt.Errorf("failed to get network status: %w", err)
	}

	fmt.Printf("\n%s  Network: %s\n", bold("●"), bold(network.Name))
	fmt.Printf("%s  %-20s %s\n", dim("  "), dim("ID:"), dim(network.ID))
	fmt.Printf("%s  %-20s %s\n", dim("  "), dim("CIDR:"), dim(network.Cidr))
	fmt.Printf("%s  %-20s %d\n", dim("  "), dim("Nodes:"), network.NodeCount)

	if len(network.Nodes) > 0 {
		fmt.Printf("\n%s  %-36s %-15s %-15s %s\n",
			dim("  "), dim("NODE ID"), dim("IP"), dim("PUBLIC KEY"), dim("STATUS"))
		fmt.Println(dim("  " + strings.Repeat("─", 90)))
		for _, n := range network.Nodes {
			nodeID, _ := n["id"].(string)
			ip, _ := n["ip_address"].(string)
			pubKey, _ := n["public_key"].(string)
			status, _ := n["status"].(string)

			statusColor := green
			if status != "online" {
				statusColor = dim
			}

			pk := pubKey
			if len(pk) > 16 {
				pk = pk[:16] + "..."
			}

			fmt.Printf("  %-36s %-15s %-15s %s\n",
				dim(nodeID),
				dim(ip),
				dim(pk),
				statusColor(status),
			)
		}
	}
	fmt.Println()
	return nil
}
