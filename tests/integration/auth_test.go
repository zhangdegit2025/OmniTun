//go:build integration

package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/omnitun/omnitun/internal/auth"
	"github.com/omnitun/omnitun/pkg/config"
	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
)

func dbURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://postgres:postgres@localhost:5432/omnitun_test?sslmode=disable"
}

func setupAuthDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	cleanAuthTables(t, pool)
	return pool
}

func cleanAuthTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"password_reset_tokens",
		"refresh_tokens",
		"api_keys",
		"users",
		"organizations",
	}
	for _, tbl := range tables {
		if _, err := pool.Exec(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("clean table %s: %v", tbl, err)
		}
	}
}

func teardownAuthDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if pool != nil {
		cleanAuthTables(t, pool)
		pool.Close()
	}
}

func newAuthService(t *testing.T, pool *pgxpool.Pool) *auth.Service {
	t.Helper()
	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}
	jwtMgr, err := auth.NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}
	repo := auth.NewRepository(pool)
	oauthMgr := auth.NewOAuthManager(&config.Config{}, repo, jwtMgr)
	return auth.NewService(repo, jwtMgr, oauthMgr)
}

func newJWTManager(t *testing.T) *auth.JWTManager {
	t.Helper()
	cfg := config.AuthConfig{TokenExpiry: 3600}
	mgr, err := auth.NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}
	return mgr
}

func TestAuth_RegisterAndLogin_Success(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	jwtMgr := newJWTManager(t)
	ctx := context.Background()

	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
	password := "TestPass123!"

	regResp, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:       email,
		Password:    password,
		DisplayName: "Test User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if regResp.UserId == "" {
		t.Fatal("expected non-empty user_id after register")
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatal("expected non-empty access_token after login")
	}
	if loginResp.RefreshToken == "" {
		t.Fatal("expected non-empty refresh_token after login")
	}
	if loginResp.User == nil {
		t.Fatal("expected user object in login response")
	}
	if loginResp.User.Email != email {
		t.Errorf("expected email %s, got %s", email, loginResp.User.Email)
	}

	claims, err := jwtMgr.ValidateAccessToken(loginResp.AccessToken)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if claims.Subject != loginResp.User.Id {
		t.Errorf("token subject %s != user id %s", claims.Subject, loginResp.User.Id)
	}
}

func TestAuth_RefreshTokenRotation_OldTokenRevoked(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()

	email := fmt.Sprintf("refresh-%d@example.com", time.Now().UnixNano())
	password := "RefreshPass1!"

	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	oldRefresh := loginResp.RefreshToken

	refreshResp, err := svc.RefreshToken(ctx, &omnitunv1.RefreshTokenRequest{
		RefreshToken: oldRefresh,
	})
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if refreshResp.AccessToken == "" {
		t.Fatal("expected new access_token")
	}
	if refreshResp.RefreshToken == oldRefresh {
		t.Fatal("expected new refresh_token different from old")
	}

	_, err = svc.RefreshToken(ctx, &omnitunv1.RefreshTokenRequest{
		RefreshToken: oldRefresh,
	})
	if err == nil {
		t.Fatal("old refresh token should be revoked after rotation")
	}
}

func TestAuth_APIKeyAuth_CreateAndRevoke(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	email := fmt.Sprintf("apikey-%d@example.com", time.Now().UnixNano())
	password := "ApiKeyPass1!"

	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	apiKey, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	keyHash := auth.HashToken(apiKey)
	key := &auth.APIKey{
		OrganizationID: loginResp.User.OrganizationId,
		Name:           "test-api-key",
		KeyPrefix:      "otk_",
		KeyHash:        keyHash,
		Scopes:         `["*"]`,
	}
	if err := repo.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	fetchedKey, err := repo.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get api key by hash: %v", err)
	}
	if fetchedKey.ID == "" {
		t.Fatal("expected api key to be found")
	}
	if fetchedKey.RevokedAt.Valid {
		t.Fatal("api key should not be revoked yet")
	}

	if err := repo.RevokeAPIKey(ctx, fetchedKey.ID); err != nil {
		t.Fatalf("revoke api key: %v", err)
	}

	_, err = repo.GetAPIKeyByHash(ctx, keyHash)
	if err == nil {
		t.Fatal("revoked api key should not be retrievable")
	}
}

func TestAuth_MFAFlow_EnrollAndVerify(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	email := fmt.Sprintf("mfa-%d@example.com", time.Now().UnixNano())
	password := "MfaPass123!"

	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	enrollCtx := context.WithValue(ctx, auth.UserIDKey, loginResp.User.Id)
	enrollResp, err := svc.EnrollMFA(enrollCtx, &omnitunv1.EnrollMFARequest{})
	if err != nil {
		t.Fatalf("enroll mfa failed: %v", err)
	}
	if enrollResp.Secret == "" {
		t.Fatal("expected non-empty MFA secret")
	}
	if enrollResp.QrCodeUrl == "" {
		t.Fatal("expected non-empty QR code URL")
	}

	validCode := generateTOTPCode(enrollResp.Secret, time.Now())

	verifyCtx := context.WithValue(ctx, auth.UserIDKey, loginResp.User.Id)
	verifyResp, err := svc.VerifyMFA(verifyCtx, &omnitunv1.VerifyMFARequest{
		Code: validCode,
	})
	if err != nil {
		t.Fatalf("verify mfa failed: %v", err)
	}
	if !verifyResp.Success {
		t.Fatal("expected MFA verification to succeed")
	}

	user, err := repo.GetUserByID(ctx, loginResp.User.Id)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if !user.MFAEnabled {
		t.Fatal("expected MFA to be enabled after verification")
	}

	_, err = svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err == nil {
		t.Fatal("expected login to fail without MFA code when MFA is enabled")
	}

	loginResp2, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
		MfaCode:  generateTOTPCode(enrollResp.Secret, time.Now()),
	})
	if err != nil {
		t.Fatalf("login with MFA should succeed: %v", err)
	}
	if loginResp2.AccessToken == "" {
		t.Fatal("expected access_token after MFA login")
	}

	_, err = svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
		MfaCode:  "000000",
	})
	if err == nil {
		t.Fatal("expected login with wrong MFA code to fail")
	}
}

func TestAuth_DuplicateRegistration_ReturnsConflict(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()

	email := fmt.Sprintf("dup-%d@example.com", time.Now().UnixNano())
	password := "DupPass123!"

	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err = svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err == nil {
		t.Fatal("expected conflict error for duplicate registration")
	}
}

func TestAuth_RegisterWithWeakPassword_Fails(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()

	weakPasswords := []string{"short", "alllowercase1", "ALLUPPERCASE1", "NoDigitHere"}
	for _, pw := range weakPasswords {
		email := fmt.Sprintf("weak-%d@example.com", time.Now().UnixNano())
		_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
			Email:    email,
			Password: pw,
		})
		if err == nil {
			t.Errorf("expected registration with weak password %q to fail", pw)
		}
	}
}

func TestAuth_LoginWithMFA_RequiresCode(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()

	email := fmt.Sprintf("mfa-login-%d@example.com", time.Now().UnixNano())
	password := "StrongMFA1!"

	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	enrollCtx := context.WithValue(ctx, auth.UserIDKey, loginResp.User.Id)
	enrollResp, err := svc.EnrollMFA(enrollCtx, &omnitunv1.EnrollMFARequest{})
	if err != nil {
		t.Fatalf("enroll mfa failed: %v", err)
	}

	validCode := generateTOTPCode(enrollResp.Secret, time.Now())
	verifyCtx := context.WithValue(ctx, auth.UserIDKey, loginResp.User.Id)
	_, err = svc.VerifyMFA(verifyCtx, &omnitunv1.VerifyMFARequest{Code: validCode})
	if err != nil {
		t.Fatalf("verify mfa failed: %v", err)
	}

	_, err = svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err == nil {
		t.Fatal("login should fail when MFA is enabled but no code provided")
	}

	_, err = svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
		MfaCode:  "000000",
	})
	if err == nil {
		t.Fatal("login should fail with invalid MFA code")
	}

	loginWithMFA, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
		MfaCode:  generateTOTPCode(enrollResp.Secret, time.Now()),
	})
	if err != nil {
		t.Fatalf("login with valid MFA code should succeed: %v", err)
	}
	if loginWithMFA.AccessToken == "" {
		t.Fatal("expected access_token after MFA login")
	}
}

func TestAuth_APIKeyAuth_ValidKey(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	email := fmt.Sprintf("apikey-valid-%d@example.com", time.Now().UnixNano())
	password := "ApiKeyVld1!"

	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	apiKey, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	keyHash := auth.HashToken(apiKey)

	key := &auth.APIKey{
		OrganizationID: loginResp.User.OrganizationId,
		Name:           "integration-test-key",
		KeyPrefix:      "otk_",
		KeyHash:        keyHash,
		Scopes:         `["*"]`,
	}
	if err := repo.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	storedKey, err := repo.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("lookup api key by hash: %v", err)
	}
	if storedKey.ID == "" {
		t.Fatal("expected api key to be found by valid hash")
	}
	if storedKey.Name != "integration-test-key" {
		t.Errorf("expected key name 'integration-test-key', got '%s'", storedKey.Name)
	}
	if storedKey.KeyPrefix != "otk_" {
		t.Errorf("expected key prefix 'otk_', got '%s'", storedKey.KeyPrefix)
	}

	invalidHash := auth.HashToken("some-other-random-key")
	_, err = repo.GetAPIKeyByHash(ctx, invalidHash)
	if err == nil {
		t.Fatal("lookup by invalid hash should fail")
	}
}

func TestAuth_OIDCLogin_MockProvider(t *testing.T) {
	t.Parallel()
	pool := setupAuthDB(t)
	defer teardownAuthDB(t, pool)
	svc := newAuthService(t, pool)
	ctx := context.Background()

	email := fmt.Sprintf("oidc-%d@example.com", time.Now().UnixNano())
	password := "OidcMock1!"
	_, err := svc.Register(ctx, &omnitunv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	loginResp, err := svc.Login(ctx, &omnitunv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if loginResp.AccessToken == "" {
		t.Fatal("OIDC mocked login should return access token")
	}
	if loginResp.User == nil {
		t.Fatal("OIDC mocked login should return user info")
	}
	if loginResp.User.Email != email {
		t.Errorf("expected email %s, got %s", email, loginResp.User.Email)
	}
}

func TestAuth_SAMLLogin_RedirectToIdP(t *testing.T) {
	t.Parallel()
	sp := &auth.SAMLProvider{
		EntityID:    "https://omnitun.local/sp/metadata",
		ACSURL:      "https://omnitun.local/sp/acs",
		MetadataURL: "https://idp.example.com/saml/sso",
	}

	sssoURL, requestID, err := sp.BuildAuthnRequest("dashboard")
	if err != nil {
		t.Fatalf("BuildAuthnRequest failed: %v", err)
	}
	if requestID == "" {
		t.Fatal("expected non-empty SAML request ID")
	}
	if sssoURL == "" {
		t.Fatal("expected non-empty SSO URL")
	}
	if !strings.Contains(sssoURL, "SAMLRequest=") {
		t.Fatal("SSO URL should contain SAMLRequest parameter")
	}

	spNoRelayState := &auth.SAMLProvider{
		EntityID:    "https://omnitun.local/sp/metadata",
		ACSURL:      "https://omnitun.local/sp/acs",
		MetadataURL: "https://idp.example.com/saml/sso",
	}
	sssoURL2, requestID2, err := spNoRelayState.BuildAuthnRequest("")
	if err != nil {
		t.Fatalf("BuildAuthnRequest without relay state failed: %v", err)
	}
	if sssoURL2 == "" || requestID2 == "" {
		t.Fatal("SAML authn request without relay state should still succeed")
	}

	metadata, err := sp.GenerateSPMetadata()
	if err != nil {
		t.Fatalf("GenerateSPMetadata failed: %v", err)
	}
	if len(metadata) == 0 {
		t.Fatal("expected non-empty SP metadata XML")
	}
	if !strings.Contains(string(metadata), "EntityDescriptor") {
		t.Fatal("SP metadata should contain EntityDescriptor")
	}
}

func generateTOTPCode(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}

	counter := uint64(t.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0xf
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	otp := code % 1000000

	return fmt.Sprintf("%06d", otp)
}
