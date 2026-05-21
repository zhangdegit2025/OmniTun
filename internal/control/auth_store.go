package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	sessionDir  = ".omnitun"
	sessionFile = "config.json"
)

type SessionFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	APIBaseURL   string `json:"api_base_url"`
	ExpiresAt    int64  `json:"expires_at"`
	Email        string `json:"email"`
}

func sessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate home directory: %w", err)
	}
	dir := filepath.Join(home, sessionDir)
	return filepath.Join(dir, sessionFile), nil
}

func LoadSession() (*SessionFile, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var s SessionFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}
	return &s, nil
}

func SaveSession(s *SessionFile) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	return nil
}

func ClearSession() error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to remove session file: %w", err)
	}
	return nil
}

func DefaultAPIURL() string {
	s, err := LoadSession()
	if err != nil || s == nil || s.APIBaseURL == "" {
		return "https://api.omnitun.io"
	}
	return s.APIBaseURL
}
