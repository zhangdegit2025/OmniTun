package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"time"
)

type Organization struct {
	ID                  string
	Name                string
	Slug                string
	Plan                string
	BillingEmail        sql.NullString
	OnboardingCompleted bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           sql.NullTime
}

type User struct {
	ID             string
	OrganizationID string
	Email          string
	PasswordHash   sql.NullString
	DisplayName    string
	AvatarURL      sql.NullString
	Role           string
	AuthProvider   string
	AuthProviderID sql.NullString
	MFAEnabled     bool
	MFASecret      sql.NullString
	LastLoginAt    sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      sql.NullTime
}

type RefreshTokenRecord struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type APIKey struct {
	ID             string
	OrganizationID string
	UserID         sql.NullString
	Name           string
	KeyPrefix      string
	KeyHash        string
	Scopes         string
	WorkspaceID    sql.NullString
	ExpiresAt      sql.NullTime
	LastUsedAt     sql.NullTime
	CreatedAt      time.Time
	RevokedAt      sql.NullTime
}

type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    sql.NullTime
	CreatedAt time.Time
}

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func VerifyTokenHash(token, hash string) bool {
	computedHash := HashToken(token)
	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(hash)) == 1
}
