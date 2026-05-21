package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <tunnel>",
		Short: "Show recent tunnel access logs",
		Long:  "Fetch and display recent log entries for a specific tunnel.",
		Args:  cobra.ExactArgs(1),
		RunE:  runLogs,
	}
	cmd.Flags().Bool("follow", false, "Follow log output (poll every 2 seconds)")
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	tunnelID := args[0]
	follow, _ := cmd.Flags().GetBool("follow")
	useJSON := isJSON(cmd)

	_, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	show := func() error {
		logs, err := client.GetTunnelLogs(tunnelID)
		if err != nil {
			return fmt.Errorf("failed to fetch logs: %w", err)
		}

		if useJSON {
			for _, l := range logs {
				fmt.Printf(`{"time":"%s","method":"%s","path":"%s","status":%d,"duration_ms":%d,"client_ip":"%s","bytes":%d}`+"\n",
					l.Timestamp, l.Method, l.Path, l.StatusCode, l.DurationMs, l.ClientIP, l.Bytes)
			}
			return nil
		}

		if len(logs) == 0 {
			fmt.Println(dim("No log entries found."))
			return nil
		}

		fmt.Printf("\n%s  %-20s %-6s %-30s %-6s %-10s %-15s\n",
			dim("  "), dim("TIME"), dim("METHOD"), dim("PATH"), dim("STATUS"), dim("DURATION"), dim("CLIENT IP"))
		fmt.Println(dim("  " + strings.Repeat("─", 90)))

		for _, l := range logs {
			statusColor := green
			if l.StatusCode >= 400 {
				statusColor = yellow
			}
			if l.StatusCode >= 500 {
				statusColor = red
			}
			ts := l.Timestamp
			if len(ts) > 19 {
				ts = ts[11:19]
			}
			duration := fmt.Sprintf("%dms", l.DurationMs)
			if l.DurationMs >= 1000 {
				duration = fmt.Sprintf("%.1fs", float64(l.DurationMs)/1000)
			}
			method := l.Method
			if method == "" {
				method = "-"
			}
			fmt.Printf("  %-20s %-6s %-30s %-6s %-10s %-15s\n",
				dim(ts),
				bold(method),
				l.Path,
				statusColor(fmt.Sprintf("%d", l.StatusCode)),
				dim(duration),
				dim(l.ClientIP),
			)
		}
		fmt.Println()
		return nil
	}

	if !follow {
		return show()
	}

	if err := show(); err != nil {
		return err
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		fmt.Print("\033[H\033[2J")
		if err := show(); err != nil {
			fmt.Printf("%s %v\n", yellow("!"), err)
		}
	}
	return nil
}
