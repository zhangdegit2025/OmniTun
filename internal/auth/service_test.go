package auth

import (
	"testing"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid password", "Abcd1234", false},
		{"valid complex", "MyP@ssw0rd!", false},
		{"too short", "Ab1", true},
		{"no uppercase", "abcd1234", true},
		{"no lowercase", "ABCD1234", true},
		{"no digit", "Abcdefgh", true},
		{"empty", "", true},
		{"exactly 8 with all required", "Abcdef12", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	tests := []string{
		"MySecureP@ss1",
		"An0ther!Pass",
		"Simple1234Test",
	}

	for _, pw := range tests {
		t.Run(pw, func(t *testing.T) {
			hash, err := HashPassword(pw)
			if err != nil {
				t.Fatalf("HashPassword(%q) error: %v", pw, err)
			}
			if hash == "" {
				t.Fatal("HashPassword returned empty hash")
			}
			if hash == pw {
				t.Fatal("hash equals plaintext password")
			}

			err = CheckPassword(hash, pw)
			if err != nil {
				t.Errorf("CheckPassword failed for correct password: %v", err)
			}

			err = CheckPassword(hash, "WrongPassword1")
			if err == nil {
				t.Error("CheckPassword should fail for wrong password")
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	token2, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	if token1 == "" {
		t.Error("GenerateToken returned empty token")
	}
	if token1 == token2 {
		t.Error("consecutive tokens should be unique")
	}
	if len(token1) < 32 {
		t.Errorf("token too short: %d chars", len(token1))
	}
}

func TestHashToken(t *testing.T) {
	token := "test-refresh-token-12345"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	if hash1 == "" {
		t.Error("HashToken returned empty hash")
	}
	if hash1 != hash2 {
		t.Error("HashToken should be deterministic")
	}
	if hash1 == token {
		t.Error("hash should not equal token")
	}
}

func TestVerifyTokenHash(t *testing.T) {
	token := "my-secret-token"
	hash := HashToken(token)

	if !VerifyTokenHash(token, hash) {
		t.Error("VerifyTokenHash should succeed for correct token")
	}
	if VerifyTokenHash("wrong-token", hash) {
		t.Error("VerifyTokenHash should fail for wrong token")
	}
}

func TestGenerateTOTPSecret(t *testing.T) {
	secret, qrURL, err := GenerateTOTPSecret("test@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret error: %v", err)
	}
	if secret == "" {
		t.Error("secret is empty")
	}
	if qrURL == "" {
		t.Error("qrURL is empty")
	}
	if len(secret) < 16 {
		t.Errorf("secret too short: %d", len(secret))
	}
}

func TestValidateTOTPCode(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("test@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPSecret error: %v", err)
	}

	valid := ValidateTOTPCode(secret, GenerateCurrentTOTP(secret))
	if !valid {
		t.Error("ValidateTOTPCode should accept current valid code")
	}

	if ValidateTOTPCode(secret, "000000") {
		t.Error("ValidateTOTPCode should reject random code")
	}
	if ValidateTOTPCode(secret, "12345") {
		t.Error("ValidateTOTPCode should reject short code")
	}
	if ValidateTOTPCode(secret, "1234567") {
		t.Error("ValidateTOTPCode should reject long code")
	}
}

func GenerateCurrentTOTP(secret string) string {
	return totpCode(secret, time.Now())
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"user@example.com", false},
		{"test.user@sub.domain.com", false},
		{"", true},
		{"notanemail", true},
		{"@example.com", true},
		{"user@", true},
		{"user name@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := validateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestEmailToSlug(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"user@example.com", "user-example-com"},
		{"test.user@sub.domain.com", "test-user-sub-domain-com"},
		{"simple@test.com", "simple-test-com"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := emailToSlug(tt.email)
			if got != tt.want {
				t.Errorf("emailToSlug(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

func TestJWTManagerIssueAndValidate(t *testing.T) {
	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager error: %v", err)
	}

	userID := "test-user-123"
	orgID := "test-org-456"
	role := "owner"

	token, err := mgr.IssueAccessToken(userID, orgID, role)
	if err != nil {
		t.Fatalf("IssueAccessToken error: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken error: %v", err)
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
}

func TestJWTManagerValidateInvalidToken(t *testing.T) {
	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager error: %v", err)
	}

	_, err = mgr.ValidateAccessToken("invalid-token")
	if err == nil {
		t.Error("ValidateAccessToken should fail for invalid token")
	}

	_, err = mgr.ValidateAccessToken("")
	if err == nil {
		t.Error("ValidateAccessToken should fail for empty token")
	}
}

func TestJWTManagerIssueRefreshToken(t *testing.T) {
	cfg := config.AuthConfig{
		TokenExpiry: 3600,
	}

	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager error: %v", err)
	}

	token1, err := mgr.IssueRefreshToken("user-1")
	if err != nil {
		t.Fatalf("IssueRefreshToken error: %v", err)
	}
	token2, err := mgr.IssueRefreshToken("user-1")
	if err != nil {
		t.Fatalf("IssueRefreshToken error: %v", err)
	}

	if token1 == "" || token2 == "" {
		t.Error("refresh token is empty")
	}
	if token1 == token2 {
		t.Error("consecutive refresh tokens should be unique")
	}
}

func TestGenerateJTI(t *testing.T) {
	jti1 := generateJTI()
	jti2 := generateJTI()

	if jti1 == "" {
		t.Error("jti is empty")
	}
	if jti1 == jti2 {
		t.Error("consecutive JTIs should be unique")
	}
}

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"user@example.com", "user-example-com"},
		{"Test.User@domain.org", "Test-User-domain-org"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := sanitizeSlug(tt.email)
			if got != tt.want {
				t.Errorf("sanitizeSlug(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}
