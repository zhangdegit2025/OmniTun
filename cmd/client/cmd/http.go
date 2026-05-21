package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/omnitun/omnitun/internal/control"
	"github.com/spf13/cobra"
)

const (
	maxRetries = 3
	retryDelay = 2 * time.Second
)

func NewHTTPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http [local_port or host:port]",
		Short: "Expose a local HTTP service via OmniTun tunnel",
		Long: `Create and start an HTTP tunnel that exposes a local service to the internet.

Examples:
  omnitun http 8080
  omnitun http 3000 --domain myapp.omnitun.io
  omnitun http 127.0.0.1:8080 --auth basic:admin:pass123`,
		Args: cobra.ExactArgs(1),
		RunE: runHTTP,
	}

	cmd.Flags().String("domain", "", "Custom domain for the tunnel")
	cmd.Flags().String("auth", "", "Basic auth protection (format: basic:user:pass)")

	return cmd
}

func runHTTP(cmd *cobra.Command, args []string) error {
	apiURL := getAPIURL(cmd)

	session, client, err := ensureSession(apiURL)
	if err != nil {
		return err
	}

	localHost, localPort, err := parseHostPort(args[0])
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", args[0], err)
	}

	domain, _ := cmd.Flags().GetString("domain")
	auth, _ := cmd.Flags().GetString("auth")

	fmt.Printf("%s Creating tunnel for %s ...\n", dim("→"), bold(fmt.Sprintf("http://%s:%d", localHost, localPort)))

	tunnel, err := client.CreateTunnel("http", localHost, localPort, domain, auth)
	if err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	publicURL := tunnel.PublicURL
	if publicURL == "" {
		publicURL = tunnel.Domain
	}

	fmt.Printf("%s Starting tunnel %s ...\n", dim("→"), bold(tunnel.ID[:8]))

	var startResp *control.StartTunnelResponse
	for attempt := 1; attempt <= maxRetries; attempt++ {
		startResp, err = client.StartTunnel(context.Background(), tunnel.ID)
		if err == nil {
			break
		}
		if attempt < maxRetries {
			fmt.Printf("%s Tunnel start attempt %d/%d failed: %v (retrying in %s...)\n",
				yellow("!"), attempt, maxRetries, err, retryDelay)
			time.Sleep(retryDelay)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to start tunnel after %d attempts: %w", maxRetries, err)
	}

	fmt.Printf("%s Tunnel started: relay=%s token=%s\n",
		dim("→"), dim(startResp.RelayAddress), dim(startResp.TunnelToken[:8]+"..."))

	tc := control.NewTunnelConnection(tunnel.ID, startResp.RelayAddress, startResp.TunnelToken, localHost, localPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tc.Establish(ctx); err != nil {
		return fmt.Errorf("failed to establish data channel: %w", err)
	}

	cfg := &control.AgentClientConfig{
		ControlWSURL: wsURLFromAPI(apiURL),
		AgentID:      "",
		Token:        session.AccessToken,
		Version:      "1.0.0",
	}

	handler := &httpTunnelHandler{
		tunnelID:   tunnel.ID,
		tunnelConn: tc,
	}
	agent := control.NewAgentClient(cfg, handler)
	agent.AddTunnelConnection(tc)

	if err := agent.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect agent: %w", err)
	}

	go tc.ForwardLoop(ctx)

	fmt.Printf("\n%s Tunnel created: %s %s %s\n",
		green("✓"),
		bold(cyan(publicURL)),
		dim("→"),
		dim(fmt.Sprintf("http://%s:%d", localHost, localPort)),
	)
	fmt.Printf("%s Tunnel is live. Press Ctrl+C to stop.\n\n", dim("●"))

	statsTicker := time.NewTicker(5 * time.Second)
	defer statsTicker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			fmt.Printf("\n%s Shutting down tunnel...\n", yellow("→"))
			_ = client.StopTunnel(tunnel.ID)
			tc.Close()
			agent.Disconnect()
			fmt.Printf("%s Tunnel stopped\n", green("✓"))
			return nil
		case <-statsTicker.C:
			tunnels, err := client.ListTunnels()
			if err != nil {
				continue
			}
			for _, t := range tunnels {
				if t.ID == tunnel.ID {
					fmt.Printf("\r%s %s | In: %s | Out: %s | Uptime: %s",
						green("●"),
						dim(publicURL),
						formatBytes(t.BytesIn),
						formatBytes(t.BytesOut),
						formatUptime(t.CreatedAt),
					)
				}
			}
		}
	}
}

func ensureSession(apiURL string) (*control.SessionFile, *control.APIClient, error) {
	session, err := control.LoadSession()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		return nil, nil, fmt.Errorf("not logged in. Run '%s login' first", os.Args[0])
	}

	if time.Now().Unix() > session.ExpiresAt && session.ExpiresAt > 0 {
		fmt.Printf("%s Session expired. Please re-login.\n", yellow("!"))

		fmt.Print("Email: ")
		reader := bufio.NewReader(os.Stdin)
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		fmt.Print("Password: ")
		password, err := readPassword()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		if email == "" || password == "" {
			return nil, nil, fmt.Errorf("email and password are required")
		}

		client := control.NewAPIClient(apiURL, "")
		resp, err := client.Login(email, password)
		if err != nil {
			return nil, nil, fmt.Errorf("re-login failed: %w", err)
		}

		session = &control.SessionFile{
			AccessToken:  resp.AccessToken,
			RefreshToken: resp.RefreshToken,
			APIBaseURL:   apiURL,
			ExpiresAt:    time.Now().Unix() + resp.ExpiresIn,
			Email:        resp.Email,
		}

		if err := control.SaveSession(session); err != nil {
			return nil, nil, fmt.Errorf("failed to save session: %w", err)
		}

		fmt.Printf("%s Re-logged in as %s\n", green("✓"), bold(resp.Email))
	}

	client := control.NewAPIClient(apiURL, session.AccessToken)
	return session, client, nil
}

type httpTunnelHandler struct {
	tunnelID   string
	tunnelConn *control.TunnelConnection
}

func (h *httpTunnelHandler) OnTunnelStartAck(msg *control.TunnelStartAck) {
	if h.tunnelConn != nil {
		return
	}
	tc := control.NewTunnelConnection(msg.TunnelID, msg.RelayAddress, msg.Token, "", 0)
	ctx := context.Background()
	if err := tc.Establish(ctx); err != nil {
		fmt.Printf("%s Failed to establish data channel: %v\n", red("✗"), err)
		return
	}
	go tc.ForwardLoop(ctx)
}

func (h *httpTunnelHandler) OnTunnelStopCmd(msg *control.TunnelStopCmd) {
	fmt.Printf("%s Tunnel %s stop command received\n", yellow("!"), msg.TunnelID[:8])
	if h.tunnelConn != nil {
		h.tunnelConn.Close()
	}
}

func (h *httpTunnelHandler) OnTunnelConfig(msg *control.TunnelConfig) {
	fmt.Printf("%s Tunnel %s config update received\n", dim("→"), msg.TunnelID[:8])
}

func (h *httpTunnelHandler) OnTunnelUpdate(msg *control.TunnelUpdate) {
	fmt.Printf("%s Tunnel %s update received\n", dim("→"), msg.TunnelID[:8])
}

func (h *httpTunnelHandler) OnShutdown(msg *control.ShutdownMessage) {
	fmt.Printf("%s Server shutdown: %s\n", red("✗"), msg.Reason)
}

func parseHostPort(input string) (string, int, error) {
	if !strings.Contains(input, ":") {
		port, err := strconv.Atoi(input)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %s", input)
		}
		return "127.0.0.1", port, nil
	}

	host, portStr, err := splitHostPort(input)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", portStr)
	}
	return host, port, nil
}

func splitHostPort(input string) (string, string, error) {
	colonIdx := strings.LastIndex(input, ":")
	if colonIdx < 0 {
		return "", "", fmt.Errorf("no port separator found")
	}
	return input[:colonIdx], input[colonIdx+1:], nil
}

func wsURLFromAPI(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil {
		return control.DefaultControlWS
	}
	wsHost := strings.Replace(u.Host, "api.", "control.", 1)
	if u.Scheme == "https" {
		return "wss://" + wsHost + "/agent/v1/connect"
	}
	return "ws://" + wsHost + "/agent/v1/connect"
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}

func formatUptime(createdAt int64) string {
	d := time.Since(time.Unix(createdAt, 0))
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
