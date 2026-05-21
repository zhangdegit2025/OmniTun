package config

import (
	"os"
	"path/filepath"
	"testing"
)

func tempConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestLoad_FromFile_ValidConfig(t *testing.T) {
	t.Parallel()

	yamlContent := `
database_url: "postgres://user:pass@localhost:5432/omnitun"
server_port: 8080
relay_port: 8443
log_level: "debug"
auth:
  token_expiry: 3600
  jwt_secret: "test-secret"
`
	path := tempConfigFile(t, yamlContent)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/omnitun" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want 8080", cfg.ServerPort)
	}
	if cfg.RelayPort != 8443 {
		t.Errorf("RelayPort = %d, want 8443", cfg.RelayPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.Auth.TokenExpiry != 3600 {
		t.Errorf("Auth.TokenExpiry = %d, want 3600", cfg.Auth.TokenExpiry)
	}
}

func TestLoad_FileNotFound_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load should fail for missing file")
	}
}

func TestValidate_MissingDatabaseURL_Fails(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		ServerPort: 8080,
		RelayPort:  8443,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate should fail when database_url is missing")
	}
}

func TestValidate_InvalidServerPort_Fails(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseURL: "postgres://localhost/db",
		ServerPort:  0,
		RelayPort:   8443,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate should fail for port 0")
	}

	cfg.ServerPort = 99999
	err = cfg.Validate()
	if err == nil {
		t.Error("Validate should fail for port > 65535")
	}
}

func TestValidate_InvalidRelayPort_Fails(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseURL: "postgres://localhost/db",
		ServerPort:  8080,
		RelayPort:   0,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate should fail for relay port 0")
	}
}

func TestValidate_ValidConfig_Passes(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseURL: "postgres://user:pass@localhost:5432/omnitun",
		ServerPort:  8080,
		RelayPort:   8443,
		LogLevel:    "info",
		Auth: AuthConfig{
			TokenExpiry: 86400,
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate should pass for valid config: %v", err)
	}
}

func TestDefaultValues_EmptyLogLevel_DefaultsToInfo(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseURL: "postgres://localhost/db",
		ServerPort:  8080,
		RelayPort:   8443,
		LogLevel:    "",
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestDefaultValues_ZeroTokenExpiry_DefaultsTo86400(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseURL: "postgres://localhost/db",
		ServerPort:  8080,
		RelayPort:   8443,
		Auth: AuthConfig{
			TokenExpiry: 0,
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if cfg.Auth.TokenExpiry != 86400 {
		t.Errorf("TokenExpiry = %d, want 86400", cfg.Auth.TokenExpiry)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	t.Parallel()

	path := tempConfigFile(t, "this: is: not: valid: yaml: [")

	_, err := Load(path)
	if err == nil {
		t.Error("Load should fail for invalid YAML")
	}
}

func TestMustLoad_PanicsOnMissingFile(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustLoad should panic on missing file")
		}
	}()

	MustLoad("/nonexistent/path/config.yaml")
}

func TestAuthConfig_Fields(t *testing.T) {
	t.Parallel()

	ac := AuthConfig{
		JWTSecret:     "secret",
		TokenExpiry:   7200,
		GitHubClientID: "gh-client-id",
	}

	if ac.JWTSecret != "secret" {
		t.Errorf("JWTSecret = %q", ac.JWTSecret)
	}
	if ac.TokenExpiry != 7200 {
		t.Errorf("TokenExpiry = %d", ac.TokenExpiry)
	}
	if ac.GitHubClientID != "gh-client-id" {
		t.Errorf("GitHubClientID = %q", ac.GitHubClientID)
	}
}
