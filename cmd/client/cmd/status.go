package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show OmniTun agent status",
		Long:  "Display version info, logged-in user, plan, active tunnels count, and traffic stats.",
		RunE:  runStatus,
	}
	return cmd
}

type statusOutput struct {
	Version           string         `json:"version"`
	GoVersion         string         `json:"go_version"`
	Platform          string         `json:"platform"`
	User              string         `json:"user"`
	Plan              string         `json:"plan"`
	TunnelCount       int            `json:"tunnel_count"`
	ActiveTunnelCount int            `json:"active_tunnel_count"`
	TrafficIn         int64          `json:"traffic_in"`
	TrafficOut        int64          `json:"traffic_out"`
	Tunnels           []tunnelOutput `json:"tunnels"`
}

type tunnelOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	URL       string `json:"url"`
	Protocol  string `json:"protocol"`
	BytesIn   int64  `json:"bytes_in"`
	BytesOut  int64  `json:"bytes_out"`
	CreatedAt string `json:"created_at"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)
	useJSON := isJSON(cmd)

	session, client, err := loadSessionAndClient(apiURL)
	if err != nil {
		return err
	}

	status, err := client.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to fetch status: %w", err)
	}

	if useJSON {
		so := statusOutput{
			Version:           Version,
			GoVersion:         runtime.Version(),
			Platform:          runtime.GOOS + "/" + runtime.GOARCH,
			User:              session.Email,
			Plan:              status.Plan,
			TunnelCount:       status.TunnelCount,
			ActiveTunnelCount: status.ActiveTunnelCount,
			TrafficIn:         status.TrafficIn,
			TrafficOut:        status.TrafficOut,
			Tunnels:           make([]tunnelOutput, 0, len(status.Tunnels)),
		}
		for _, t := range status.Tunnels {
			so.Tunnels = append(so.Tunnels, tunnelOutput{
				ID:        t.ID,
				Name:      t.Name,
				Status:    t.Status,
				URL:       t.PublicURL,
				Protocol:  t.Protocol,
				BytesIn:   t.BytesIn,
				BytesOut:  t.BytesOut,
				CreatedAt: fmt.Sprintf("%d", t.CreatedAt),
			})
		}
		data, _ := json.MarshalIndent(so, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("\n%s  OmniTun v%s %s\n", bold("●"), cyan(Version), dim("("+Commit[:8]+")"))
	fmt.Printf("%s  \n", dim(strings.Repeat("─", 50)))
	fmt.Printf("%s  %-20s %s\n", dim("  "), dim("User:"), bold(session.Email))
	fmt.Printf("%s  %-20s %s\n", dim("  "), dim("Plan:"), bold(status.Plan))
	fmt.Printf("%s  %-20s %d (%d active)\n", dim("  "), dim("Tunnels:"), status.TunnelCount, status.ActiveTunnelCount)
	fmt.Printf("%s  %-20s %s / %s\n", dim("  "), dim("Traffic:"), formatBytes(status.TrafficIn)+" in", formatBytes(status.TrafficOut)+" out")

	if len(status.Tunnels) > 0 {
		fmt.Printf("\n%s  %-36s %-10s %s\n",
			dim("  "), dim("NAME"), dim("STATUS"), dim("URL"))
		fmt.Println(dim("  " + strings.Repeat("─", 80)))
		for _, t := range status.Tunnels {
			statusColor := green
			if t.Status != "active" {
				statusColor = dim
			}
			url := t.PublicURL
			if url == "" {
				url = t.Domain
			}
			name := t.Name
			if name == "" {
				name = t.ID[:8]
			}
			fmt.Printf("  %-36s %-10s %s\n",
				dim(name),
				statusColor(t.Status),
				dim(url),
			)
		}
	}
	fmt.Println()
	return nil
}
