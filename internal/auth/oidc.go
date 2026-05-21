package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

type OIDCProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	DiscoveryURL string
	isMock       bool

	authEndpoint     string
	tokenEndpoint    string
	userinfoEndpoint string
	discovered       bool
}

type OIDCUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Sub   string `json:"sub"`
}

const mockOIDCCode = "__mock_oidc_code__"

func NewOIDCProvider(cfg config.AuthConfig) *OIDCProvider {
	p := &OIDCProvider{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		DiscoveryURL: cfg.OIDCDiscoveryURL,
	}
	if p.DiscoveryURL == "" || strings.HasPrefix(p.DiscoveryURL, "mock://") {
		p.isMock = true
	}
	return p
}

func (p *OIDCProvider) IsConfigured() bool {
	if p.isMock {
		return true
	}
	return p.ClientID != "" && p.ClientSecret != "" && p.DiscoveryURL != "" && p.RedirectURL != ""
}

func (p *OIDCProvider) AuthCodeURL(state string) (string, error) {
	if p.isMock {
		return fmt.Sprintf("%s?code=%s&state=%s", p.RedirectURL, mockOIDCCode, url.QueryEscape(state)), nil
	}

	if err := p.discover(); err != nil {
		return "", fmt.Errorf("OIDC discovery failed: %w", err)
	}

	u, err := url.Parse(p.authEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid auth endpoint: %w", err)
	}
	q := u.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*OIDCUser, error) {
	if p.isMock {
		if code != mockOIDCCode {
			return nil, fmt.Errorf("invalid mock OIDC code")
		}
		return &OIDCUser{
			Email: "oidc-user@example.com",
			Name:  "OIDC Test User",
			Sub:   "mock-oidc-sub-001",
		}, nil
	}

	if err := p.discover(); err != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", err)
	}

	tokenResp, err := p.exchangeToken(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return p.fetchUserInfo(ctx, tokenResp.AccessToken)
}

func (p *OIDCProvider) discover() error {
	if p.discovered {
		return nil
	}

	resp, err := http.Get(p.DiscoveryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer resp.Body.Close()

	var discovery struct {
		AuthEndpoint    string `json:"authorization_endpoint"`
		TokenEndpoint   string `json:"token_endpoint"`
		UserinfoEndpoint string `json:"userinfo_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return fmt.Errorf("failed to decode discovery document: %w", err)
	}

	p.authEndpoint = discovery.AuthEndpoint
	p.tokenEndpoint = discovery.TokenEndpoint
	p.userinfoEndpoint = discovery.UserinfoEndpoint
	p.discovered = true
	return nil
}

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

func (p *OIDCProvider) exchangeToken(ctx context.Context, code string) (*oidcTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", p.RedirectURL)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp oidcTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w (body: %s)", err, string(body))
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in response: %s", string(body))
	}
	return &tokenResp, nil
}

func (p *OIDCProvider) fetchUserInfo(ctx context.Context, accessToken string) (*OIDCUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	var user OIDCUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}
	return &user, nil
}

type OIDCManager struct {
	provider *OIDCProvider
	repo     Repository
	jwtMgr   *JWTManager
}

func NewOIDCManager(cfg config.AuthConfig, repo Repository, jwtMgr *JWTManager) *OIDCManager {
	return &OIDCManager{
		provider: NewOIDCProvider(cfg),
		repo:     repo,
		jwtMgr:   jwtMgr,
	}
}

func (m *OIDCManager) IsMock() bool {
	return m.provider.isMock
}

func (m *OIDCManager) AuthCodeURL(state string) (string, error) {
	return m.provider.AuthCodeURL(state)
}

func (m *OIDCManager) HandleCallback(ctx context.Context, code string) (*User, string, string, error) {
	oidcUser, err := m.provider.Exchange(ctx, code)
	if err != nil {
		return nil, "", "", fmt.Errorf("OIDC exchange failed: %w", err)
	}

	user, err := m.findOrCreateOIDCUser(ctx, oidcUser)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to find or create OIDC user: %w", err)
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

func (m *OIDCManager) findOrCreateOIDCUser(ctx context.Context, oidcUser *OIDCUser) (*User, error) {
	user, err := m.repo.GetUserByProvider(ctx, "oidc", oidcUser.Sub)
	if err == nil {
		return user, nil
	}

	if oidcUser.Email != "" {
		user, err = m.repo.FindUserByEmail(ctx, oidcUser.Email)
		if err == nil {
			return user, nil
		}
	}

	org := &Organization{
		Name: oidcUser.Email,
		Slug: oidcSlug(oidcUser.Email),
		Plan: "free",
	}
	if err := m.repo.CreateOrganization(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	user = &User{
		OrganizationID: org.ID,
		Email:          oidcUser.Email,
		DisplayName:    oidcUser.Name,
		Role:           "owner",
		AuthProvider:   "oidc",
	}
	user.AuthProviderID = sql.NullString{String: oidcUser.Sub, Valid: true}

	if err := m.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func oidcSlug(email string) string {
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

func GenerateOIDCState() (string, error) {
	return GenerateToken()
}

func SignOIDCState(state, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(state))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyOIDCState(state, signature, secret string) bool {
	expected := SignOIDCState(state, secret)
	return hmac.Equal([]byte(signature), []byte(expected))
}
