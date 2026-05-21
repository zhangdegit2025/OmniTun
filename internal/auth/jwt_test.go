package auth

import (
	"testing"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

func TestJWTManager_IssueAndValidate_ClaimsConsistent(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 7200,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	userID := "user-abc-123"
	orgID := "org-def-456"
	role := "admin"

	token, err := mgr.IssueAccessToken(userID, orgID, role)
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if claims == nil {
		t.Fatal("claims is nil")
	}
	if claims.Subject != userID {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID)
	}
	if claims.OrgID != orgID {
		t.Errorf("OrgID = %q, want %q", claims.OrgID, orgID)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
	if claims.IssuedAt == nil {
		t.Error("IssuedAt should not be nil")
	}
	if claims.ID == "" {
		t.Error("JTI should not be empty")
	}
}

func TestJWTManager_ExpiredToken_ValidationFails(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 1,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	token, err := mgr.IssueAccessToken("user-1", "org-1", "viewer")
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	time.Sleep(2 * time.Second)

	_, err = mgr.ValidateAccessToken(token)
	if err == nil {
		t.Error("ValidateAccessToken should fail for expired token")
	}
}

func TestJWTManager_InvalidSignature_ValidationFails(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	token, err := mgr.IssueAccessToken("user-1", "org-1", "viewer")
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	corruptedToken := token + "tampered"

	_, err = mgr.ValidateAccessToken(corruptedToken)
	if err == nil {
		t.Error("ValidateAccessToken should fail for tampered token")
	}
}

func TestJWTManager_RefreshTokenRotation(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	token1, err := mgr.IssueRefreshToken("user-rot")
	if err != nil {
		t.Fatalf("first IssueRefreshToken failed: %v", err)
	}
	if token1 == "" {
		t.Fatal("first refresh token is empty")
	}

	token2, err := mgr.IssueRefreshToken("user-rot")
	if err != nil {
		t.Fatalf("second IssueRefreshToken failed: %v", err)
	}
	if token2 == "" {
		t.Fatal("second refresh token is empty")
	}

	if token1 == token2 {
		t.Error("consecutive refresh tokens should be different")
	}
}

func TestJWTManager_HS256WithSecret(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		JWTSecret:   "test-secret-key-for-HS256-32bytes!",
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager with secret failed: %v", err)
	}

	token, err := mgr.IssueAccessToken("hs-user", "hs-org", "member")
	if err != nil {
		t.Fatalf("IssueAccessToken (HS256) failed: %v", err)
	}
	if token == "" {
		t.Fatal("HS256 token is empty")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken (HS256) failed: %v", err)
	}
	if claims.Subject != "hs-user" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "hs-user")
	}
}

func TestJWTManager_ValidateEmptyToken(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	_, err = mgr.ValidateAccessToken("")
	if err == nil {
		t.Error("ValidateAccessToken should fail for empty token")
	}
}

func TestJWTManager_ValidateBogusToken(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}

	_, err = mgr.ValidateAccessToken("not.a.jwt.at.all")
	if err == nil {
		t.Error("ValidateAccessToken should fail for bogus token")
	}
}

func TestJWTManager_NoKeysNoSecret_PanicsOrErrors(t *testing.T) {
	t.Parallel()

	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager should auto-generate keys if none provided: %v", err)
	}

	_, err = mgr.IssueAccessToken("user", "org", "role")
	if err != nil {
		t.Errorf("IssueAccessToken should succeed with auto-generated RSA keys: %v", err)
	}
}
