package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultAPIURL    = "https://api.omnitun.io"
	DefaultControlWS = "wss://control.omnitun.io/agent/v1/connect"
	clientTimeout    = 30 * time.Second
)

type APIClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		Token:   token,
		HTTP: &http.Client{
			Timeout: clientTimeout,
		},
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Email        string `json:"email"`
}

type TunnelRequest struct {
	Protocol  string `json:"protocol"`
	LocalHost string `json:"local_host"`
	LocalPort int    `json:"local_port"`
	Domain    string `json:"domain,omitempty"`
	Auth      string `json:"auth,omitempty"`
}

type TunnelResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	LocalHost  string `json:"local_host"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Domain     string `json:"domain"`
	Status     string `json:"status"`
	PublicURL  string `json:"public_url"`
	BytesIn    int64  `json:"bytes_in"`
	BytesOut   int64  `json:"bytes_out"`
	CreatedAt  int64  `json:"created_at"`
}

type StartTunnelResponse struct {
	Message      string `json:"message"`
	RelayAddress string `json:"relay_address"`
	TunnelToken  string `json:"tunnel_token"`
}

type TunnelListResponse struct {
	Tunnels []TunnelResponse `json:"tunnels"`
}

type AlternativeRelayResponse struct {
	RelayID       string `json:"relay_id"`
	Region        string `json:"region"`
	Hostname      string `json:"hostname"`
	Port          int    `json:"port"`
	ActiveTunnels int    `json:"active_tunnels"`
	Capacity      int    `json:"capacity"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *APIClient) do(method, path string, body interface{}, result interface{}) error {
	url := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if json.Unmarshal(respData, &apiErr) == nil && apiErr.Message != "" {
			return fmt.Errorf("[%d] %s: %s", resp.StatusCode, apiErr.Code, apiErr.Message)
		}
		return fmt.Errorf("[%d] %s", resp.StatusCode, string(respData))
	}

	if result != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}

func (c *APIClient) Login(email, password string) (*LoginResponse, error) {
	req := &LoginRequest{
		Email:    email,
		Password: password,
	}
	var resp LoginResponse
	if err := c.do("POST", "/v1/auth/login", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *APIClient) Logout() error {
	return c.do("POST", "/v1/auth/logout", nil, nil)
}

func (c *APIClient) CreateTunnel(protocol, localHost string, localPort int, domain, auth string) (*TunnelResponse, error) {
	req := &TunnelRequest{
		Protocol:  protocol,
		LocalHost: localHost,
		LocalPort: localPort,
		Domain:    domain,
		Auth:      auth,
	}
	var resp TunnelResponse
	if err := c.do("POST", "/v1/tunnels?workspace_id=auto", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *APIClient) StartTunnel(ctx context.Context, id string) (*StartTunnelResponse, error) {
	var resp StartTunnelResponse
	if err := c.do("POST", "/v1/tunnels/"+id+"/start", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *APIClient) StopTunnel(id string) error {
	return c.do("POST", "/v1/tunnels/"+id+"/stop", nil, nil)
}

func (c *APIClient) DeleteTunnel(id string) error {
	return c.do("DELETE", "/v1/tunnels/"+id, nil, nil)
}

func (c *APIClient) ListTunnels() ([]TunnelResponse, error) {
	var resp TunnelListResponse
	if err := c.do("GET", "/v1/tunnels", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Tunnels, nil
}

func (c *APIClient) GetAlternativeRelay(region, failedRelayID string) (*AlternativeRelayResponse, error) {
	path := fmt.Sprintf("/v1/relays/alternative?region=%s&exclude=%s", region, failedRelayID)
	var resp AlternativeRelayResponse
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type StatusResponse struct {
	Version      string           `json:"version"`
	Email        string           `json:"email"`
	Plan         string           `json:"plan"`
	TunnelCount  int              `json:"tunnel_count"`
	ActiveTunnelCount int         `json:"active_tunnel_count"`
	TrafficIn    int64            `json:"traffic_in"`
	TrafficOut   int64            `json:"traffic_out"`
	Tunnels      []TunnelResponse `json:"tunnels"`
}

func (c *APIClient) GetStatus() (*StatusResponse, error) {
	var resp StatusResponse
	if err := c.do("GET", "/v1/status", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	BuildDate string `json:"build_date"`
}

func (c *APIClient) GetLatestVersion() (*VersionResponse, error) {
	var resp VersionResponse
	if err := c.do("GET", "/v1/releases/latest", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type NetworkResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cidr      string `json:"cidr"`
	NodeCount int    `json:"node_count"`
	CreatedAt int64  `json:"created_at"`
}

type NetworkListResponse struct {
	Networks []NetworkResponse `json:"networks"`
}

type NetworkStatusResponse struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Cidr      string                   `json:"cidr"`
	NodeCount int                      `json:"node_count"`
	CreatedAt int64                    `json:"created_at"`
	Nodes     []map[string]interface{} `json:"nodes"`
}

type CreateNetworkResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cidr      string `json:"cidr"`
	CreatedAt int64  `json:"created_at"`
}

type JoinNetworkResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cidr      string `json:"cidr"`
	NodeCount int    `json:"node_count"`
	CreatedAt int64  `json:"created_at"`
}

func (c *APIClient) ListNetworks() ([]NetworkResponse, error) {
	var resp NetworkListResponse
	if err := c.do("GET", "/v1/networks", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Networks, nil
}

func (c *APIClient) CreateNetwork(name string) (*CreateNetworkResponse, error) {
	req := map[string]string{"name": name}
	var resp CreateNetworkResponse
	if err := c.do("POST", "/v1/networks", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *APIClient) JoinNetwork(inviteCode string) (*JoinNetworkResponse, error) {
	req := map[string]string{"invite_code": inviteCode}
	var resp JoinNetworkResponse
	if err := c.do("POST", "/v1/networks/join", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *APIClient) LeaveNetwork(networkID string) error {
	return c.do("DELETE", "/v1/networks/"+networkID, nil, nil)
}

func (c *APIClient) GetNetworkStatus(networkID string) (*NetworkStatusResponse, error) {
	var resp NetworkStatusResponse
	if err := c.do("GET", "/v1/networks/"+networkID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type LogEntry struct {
	Timestamp   string `json:"timestamp"`
	TunnelID    string `json:"tunnel_id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	StatusCode  int    `json:"status_code"`
	DurationMs  int64  `json:"duration_ms"`
	ClientIP    string `json:"client_ip"`
	Bytes       int64  `json:"bytes"`
	Error       string `json:"error"`
}

func (c *APIClient) GetTunnelLogs(tunnelID string) ([]LogEntry, error) {
	var resp []LogEntry
	if err := c.do("GET", "/v1/tunnels/"+tunnelID+"/logs", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
