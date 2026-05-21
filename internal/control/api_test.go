package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIClient_Login_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Email != "user@example.com" {
			http.Error(w, `{"code":"INVALID_CREDENTIALS","message":"invalid email"}`, http.StatusUnauthorized)
			return
		}

		resp := LoginResponse{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-456",
			ExpiresIn:    3600,
			Email:        "user@example.com",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	resp, err := client.Login("user@example.com", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp == nil {
		t.Fatal("Login returned nil response")
	}
	if resp.AccessToken != "access-token-123" {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "access-token-123")
	}
	if resp.RefreshToken != "refresh-token-456" {
		t.Errorf("RefreshToken = %q, want %q", resp.RefreshToken, "refresh-token-456")
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", resp.ExpiresIn)
	}
	if resp.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", resp.Email, "user@example.com")
	}
}

func TestAPIClient_CreateTunnel_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tunnels" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req TunnelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Protocol != "tcp" {
			http.Error(w, `{"code":"INVALID_PROTOCOL","message":"unsupported"}`, http.StatusBadRequest)
			return
		}

		resp := TunnelResponse{
			ID:         "tun-created-001",
			Name:       "my-tunnel",
			Protocol:   req.Protocol,
			LocalHost:  req.LocalHost,
			LocalPort:  req.LocalPort,
			RemotePort: 5000,
			Domain:     req.Domain,
			Status:     "created",
			PublicURL:  "https://my-tunnel.omnitun.io",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "bearer-token")
	resp, err := client.CreateTunnel("tcp", "127.0.0.1", 8080, "my-tunnel.omnitun.io", "")
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}
	if resp == nil {
		t.Fatal("CreateTunnel returned nil response")
	}
	if resp.ID != "tun-created-001" {
		t.Errorf("ID = %q, want %q", resp.ID, "tun-created-001")
	}
	if resp.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", resp.Protocol, "tcp")
	}
	if resp.LocalPort != 8080 {
		t.Errorf("LocalPort = %d, want 8080", resp.LocalPort)
	}
	if resp.RemotePort != 5000 {
		t.Errorf("RemotePort = %d, want 5000", resp.RemotePort)
	}
	if resp.Status != "created" {
		t.Errorf("Status = %q, want %q", resp.Status, "created")
	}
}

func TestAPIClient_ErrorResponse_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(APIError{
			Code:    "UNAUTHORIZED",
			Message: "invalid credentials",
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "invalid-token")
	resp, err := client.Login("user@example.com", "wrong-password")

	if err == nil {
		t.Error("Login should fail with unauthorized error")
	}
	if resp != nil {
		t.Error("Login should return nil response on error")
	}
	if err != nil {
		t.Logf("expected error: %v", err)
	}
}

func TestAPIClient_ErrorResponse_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tunnels/tun-does-not-exist/start" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIError{
				Code:    "NOT_FOUND",
				Message: "tunnel not found",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "token")
	resp, err := client.StartTunnel(context.Background(), "tun-does-not-exist")

	if err == nil {
		t.Error("StartTunnel should fail with not found error")
	}
	if resp != nil {
		t.Error("StartTunnel should return nil response on error")
	}
	if err != nil {
		t.Logf("expected error: %v", err)
	}
}

func TestAPIClient_ListTunnels_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tunnels" {
			http.NotFound(w, r)
			return
		}
		resp := TunnelListResponse{
			Tunnels: []TunnelResponse{
				{ID: "tun-1", Protocol: "tcp", LocalPort: 3000, Status: "active"},
				{ID: "tun-2", Protocol: "http", LocalPort: 8080, Status: "active"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "token")
	tunnels, err := client.ListTunnels()

	if err != nil {
		t.Fatalf("ListTunnels failed: %v", err)
	}
	if len(tunnels) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(tunnels))
	}
	if tunnels[0].ID != "tun-1" {
		t.Errorf("tunnels[0].ID = %q, want %q", tunnels[0].ID, "tun-1")
	}
	if tunnels[1].ID != "tun-2" {
		t.Errorf("tunnels[1].ID = %q, want %q", tunnels[1].ID, "tun-2")
	}
}

func TestAPIClient_AuthorizationHeader_IncludesToken(t *testing.T) {
	t.Parallel()

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "my-bearer-token")
	_ = client.Logout()

	if authHeader != "Bearer my-bearer-token" {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer my-bearer-token")
	}
}

func TestAPIClient_StopTunnel(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tunnels/tun-001/stop" && r.Method == http.MethodPost {
			called = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "token")
	err := client.StopTunnel("tun-001")

	if err != nil {
		t.Fatalf("StopTunnel failed: %v", err)
	}
	if !called {
		t.Error("stop endpoint was not called")
	}
}

func TestAPIClient_DeleteTunnel(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tunnels/tun-001" && r.Method == http.MethodDelete {
			called = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "token")
	err := client.DeleteTunnel("tun-001")

	if err != nil {
		t.Fatalf("DeleteTunnel failed: %v", err)
	}
	if !called {
		t.Error("delete endpoint was not called")
	}
}

func TestAPIClient_ErrorResponse_BadRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"code":"BAD_REQUEST","message":"invalid request body"}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "token")
	_, err := client.CreateTunnel("", "", 0, "", "")

	if err == nil {
		t.Error("CreateTunnel should fail with bad request error")
	}
}
