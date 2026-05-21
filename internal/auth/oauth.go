package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

type OAuthManager struct {
	githubClientID     string
	githubClientSecret string
	githubRedirectURL  string
	googleClientID     string
	googleClientSecret string
	googleRedirectURL  string
	repo               Repository
	jwtMgr             *JWTManager
	logger             *slog.Logger
}

type OAuthUserInfo struct {
	Provider       string
	ProviderUserID string
	Email          string
	DisplayName    string
	AvatarURL      string
}

func NewOAuthManager(cfg *config.Config, repo Repository, jwtMgr *JWTManager) *OAuthManager {
	return &OAuthManager{
		githubClientID:     cfg.Auth.GitHubClientID,
		githubClientSecret: cfg.Auth.GitHubClientSecret,
		githubRedirectURL:  cfg.Auth.GitHubRedirectURL,
		googleClientID:     cfg.Auth.GoogleClientID,
		googleClientSecret: cfg.Auth.GoogleClientSecret,
		googleRedirectURL:  cfg.Auth.GoogleRedirectURL,
		repo:               repo,
		jwtMgr:             jwtMgr,
		logger:             slog.Default().With("component", "oauth_manager"),
	}
}

func (m *OAuthManager) GitHubLoginURL(state string) string {
	if m.githubClientID == "" {
		return ""
	}
	u, _ := url.Parse("https://github.com/login/oauth/authorize")
	q := u.Query()
	q.Set("client_id", m.githubClientID)
	q.Set("redirect_uri", m.githubRedirectURL)
	q.Set("scope", "user:email")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

func (m *OAuthManager) GoogleLoginURL(state string) string {
	if m.googleClientID == "" {
		return ""
	}
	u, _ := url.Parse("https://accounts.google.com/o/oauth2/v2/auth")
	q := u.Query()
	q.Set("client_id", m.googleClientID)
	q.Set("redirect_uri", m.googleRedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	u.RawQuery = q.Encode()
	return u.String()
}

func (m *OAuthManager) HandleOAuthCallback(ctx context.Context, provider, code string) (*User, string, string, error) {
	switch provider {
	case "github":
		return m.handleGitHubCallback(ctx, code)
	case "google":
		return m.handleGoogleCallback(ctx, code)
	default:
		return nil, "", "", fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
}

func (m *OAuthManager) handleGitHubCallback(ctx context.Context, code string) (*User, string, string, error) {
	if m.githubClientID == "" {
		return nil, "", "", fmt.Errorf("github OAuth is not configured")
	}

	accessToken, err := m.exchangeGitHubCode(ctx, code)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to exchange GitHub code: %w", err)
	}

	userInfo, err := m.fetchGitHubUserInfo(ctx, accessToken)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to fetch GitHub user info: %w", err)
	}

	return m.finalizeOAuthLogin(ctx, userInfo)
}

func (m *OAuthManager) exchangeGitHubCode(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", m.githubClientID)
	data.Set("client_secret", m.githubClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", m.githubRedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("github OAuth error: %s", result.Error)
	}
	return result.AccessToken, nil
}

func (m *OAuthManager) fetchGitHubUserInfo(ctx context.Context, accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub user: %w", err)
	}
	defer resp.Body.Close()

	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub user: %w", err)
	}

	email := ghUser.Email
	if email == "" {
		emails, err := m.fetchGitHubEmails(ctx, accessToken)
		if err == nil {
			for _, e := range emails {
				if e.Primary && e.Verified {
					email = e.Email
					break
				}
			}
		}
	}

	displayName := ghUser.Name
	if displayName == "" {
		displayName = ghUser.Login
	}

	return &OAuthUserInfo{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", ghUser.ID),
		Email:          email,
		DisplayName:    displayName,
		AvatarURL:      ghUser.AvatarURL,
	}, nil
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (m *OAuthManager) fetchGitHubEmails(ctx context.Context, accessToken string) ([]githubEmail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return nil, err
	}
	return emails, nil
}

func (m *OAuthManager) handleGoogleCallback(ctx context.Context, code string) (*User, string, string, error) {
	if m.googleClientID == "" {
		return nil, "", "", fmt.Errorf("google OAuth is not configured")
	}

	accessToken, err := m.exchangeGoogleCode(ctx, code)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to exchange Google code: %w", err)
	}

	userInfo, err := m.fetchGoogleUserInfo(ctx, accessToken)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to fetch Google user info: %w", err)
	}

	return m.finalizeOAuthLogin(ctx, userInfo)
}

func (m *OAuthManager) exchangeGoogleCode(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", m.googleClientID)
	data.Set("client_secret", m.googleClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", m.googleRedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Google token response: %w (body: %s)", err, string(body))
	}
	if result.Error != "" {
		return "", fmt.Errorf("google OAuth error: %s", result.Error)
	}
	return result.AccessToken, nil
}

func (m *OAuthManager) fetchGoogleUserInfo(ctx context.Context, accessToken string) (*OAuthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v2/userinfo", nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google user info: %w", err)
	}
	defer resp.Body.Close()

	var gUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gUser); err != nil {
		return nil, fmt.Errorf("failed to decode Google user info: %w", err)
	}

	return &OAuthUserInfo{
		Provider:       "google",
		ProviderUserID: gUser.ID,
		Email:          gUser.Email,
		DisplayName:    gUser.Name,
		AvatarURL:      gUser.Picture,
	}, nil
}

func (m *OAuthManager) finalizeOAuthLogin(ctx context.Context, info *OAuthUserInfo) (*User, string, string, error) {
	user, err := m.findOrCreateOAuthUser(ctx, info)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to find or create OAuth user: %w", err)
	}

	accessToken, err := m.jwtMgr.IssueAccessToken(user.ID, user.OrganizationID, user.Role)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to issue access token: %w", err)
	}
	refreshToken, err := m.jwtMgr.IssueRefreshToken(user.ID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to issue refresh token: %w", err)
	}

	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
	if err := m.repo.StoreRefreshToken(ctx, user.ID, refreshToken, refreshExpiry); err != nil {
		return nil, "", "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	return user, accessToken, refreshToken, nil
}

func (m *OAuthManager) findOrCreateOAuthUser(ctx context.Context, info *OAuthUserInfo) (*User, error) {
	user, err := m.repo.GetUserByProvider(ctx, info.Provider, info.ProviderUserID)
	if err == nil {
		return user, nil
	}

	user, err = m.repo.FindUserByEmail(ctx, info.Email)
	if err == nil {
		return user, nil
	}

	org := &Organization{
		Name: info.Email,
		Slug: sanitizeSlug(info.Email),
		Plan: "free",
	}
	if err := m.repo.CreateOrganization(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	user = &User{
		OrganizationID: org.ID,
		Email:          info.Email,
		DisplayName:    info.DisplayName,
		Role:           "owner",
		AuthProvider:   info.Provider,
	}
	user.AuthProviderID = sql.NullString{String: info.ProviderUserID, Valid: true}
	if info.AvatarURL != "" {
		user.AvatarURL = sql.NullString{String: info.AvatarURL, Valid: true}
	}

	if err := m.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func sanitizeSlug(email string) string {
	slug := ""
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			slug += string(r)
		} else if r == '@' || r == '.' {
			slug += "-"
		}
	}
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}
