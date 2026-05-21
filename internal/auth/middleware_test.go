package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

type mockRepository struct {
	getAPIKeyByHashFunc func(ctx context.Context, hash string) (*APIKey, error)
}

func (m *mockRepository) CreateOrganization(ctx context.Context, org *Organization) error        { return nil }
func (m *mockRepository) GetOrganization(ctx context.Context, id string) (*Organization, error)   { return nil, nil }
func (m *mockRepository) CreateUser(ctx context.Context, user *User) error                         { return nil }
func (m *mockRepository) GetUserByEmail(ctx context.Context, orgID, email string) (*User, error)   { return nil, nil }
func (m *mockRepository) FindUserByEmail(ctx context.Context, email string) (*User, error)         { return nil, nil }
func (m *mockRepository) GetUserByID(ctx context.Context, id string) (*User, error)                { return nil, nil }
func (m *mockRepository) GetUserByProvider(ctx context.Context, provider, providerID string) (*User, error) {
	return nil, nil
}
func (m *mockRepository) UpdateLastLogin(ctx context.Context, userID string) error        { return nil }
func (m *mockRepository) UpdateMFA(ctx context.Context, userID string, enabled bool, secret string) error {
	return nil
}
func (m *mockRepository) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error { return nil }
func (m *mockRepository) StoreRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	return nil
}
func (m *mockRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	return nil, nil
}
func (m *mockRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error       { return nil }
func (m *mockRepository) DeleteUserRefreshTokens(ctx context.Context, userID string) error      { return nil }
func (m *mockRepository) CreateAPIKey(ctx context.Context, key *APIKey) error                   { return nil }
func (m *mockRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	if m.getAPIKeyByHashFunc != nil {
		return m.getAPIKeyByHashFunc(ctx, hash)
	}
	return nil, nil
}
func (m *mockRepository) RevokeAPIKey(ctx context.Context, id string) error { return nil }
func (m *mockRepository) StorePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	return nil
}
func (m *mockRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	return nil, nil
}
func (m *mockRepository) ConsumePasswordResetToken(ctx context.Context, tokenHash string) error { return nil }
func (m *mockRepository) SetOnboardingCompleted(ctx context.Context, id string) error { return nil }

func newJWTManager(t *testing.T) *JWTManager {
	t.Helper()
	cfg := config.AuthConfig{TokenExpiry: 3600}
	mgr, err := NewJWTManager(cfg)
	if err != nil {
		t.Fatalf("NewJWTManager failed: %v", err)
	}
	return mgr
}

func TestJWTAuthMiddleware_ValidToken_Returns200(t *testing.T) {
	t.Parallel()

	mgr := newJWTManager(t)
	token, err := mgr.IssueAccessToken("user-1", "org-1", "admin")
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	handler := JWTAuthMiddleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserID(r.Context())
		if !ok {
			t.Error("user_id not found in context")
		}
		if userID != "user-1" {
			t.Errorf("user_id = %q, want %q", userID, "user-1")
		}
		orgID, ok := GetOrgID(r.Context())
		if !ok {
			t.Error("org_id not found in context")
		}
		if orgID != "org-1" {
			t.Errorf("org_id = %q, want %q", orgID, "org-1")
		}
		role, ok := GetRole(r.Context())
		if !ok {
			t.Error("role not found in context")
		}
		if role != "admin" {
			t.Errorf("role = %q, want %q", role, "admin")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestJWTAuthMiddleware_MissingHeader_Returns401(t *testing.T) {
	t.Parallel()

	mgr := newJWTManager(t)
	handler := JWTAuthMiddleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestJWTAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()

	mgr := newJWTManager(t)
	handler := JWTAuthMiddleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt-token-here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestJWTAuthMiddleware_InvalidHeaderFormat_Returns401(t *testing.T) {
	t.Parallel()

	mgr := newJWTManager(t)
	handler := JWTAuthMiddleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	tests := []struct {
		name  string
		value string
	}{
		{"no scheme", "sometokenwithoutbearer"},
		{"Basic scheme", "Basic dXNlcjpwYXNz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.value)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestAPIKeyAuthMiddleware_ValidKey_Returns200(t *testing.T) {
	t.Parallel()

	testAPIKey := "omnitun-api-key-valid-12345"
	keyHash := ComputeAPIKeyHash(testAPIKey)

	repo := &mockRepository{
		getAPIKeyByHashFunc: func(ctx context.Context, hash string) (*APIKey, error) {
			if hash == keyHash {
				return &APIKey{
					ID:             "key-001",
					OrganizationID: "org-001",
					UserID:         sql.NullString{String: "user-001", Valid: true},
				}, nil
			}
			return nil, nil
		},
	}

	handler := APIKeyAuthMiddleware(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeyID, ok := r.Context().Value(APIKeyIDKey).(string)
		if !ok || apiKeyID != "key-001" {
			t.Errorf("api_key_id = %q, want %q", apiKeyID, "key-001")
		}
		userID, ok := GetUserID(r.Context())
		if !ok || userID != "user-001" {
			t.Errorf("user_id = %q, want %q", userID, "user-001")
		}
		orgID, ok := GetOrgID(r.Context())
		if !ok || orgID != "org-001" {
			t.Errorf("org_id = %q, want %q", orgID, "org-001")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAPIKeyAuthMiddleware_MissingKey_Returns401(t *testing.T) {
	t.Parallel()

	repo := &mockRepository{}
	handler := APIKeyAuthMiddleware(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAPIKeyAuthMiddleware_InvalidKey_Returns401(t *testing.T) {
	t.Parallel()

	repo := &mockRepository{
		getAPIKeyByHashFunc: func(ctx context.Context, hash string) (*APIKey, error) {
			return nil, fmt.Errorf("api key not found")
		},
	}

	handler := APIKeyAuthMiddleware(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "nonexistent-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAPIKeyAuthMiddleware_NoUserID_Returns200(t *testing.T) {
	t.Parallel()

	testAPIKey := "api-key-no-user-67890"
	keyHash := ComputeAPIKeyHash(testAPIKey)

	repo := &mockRepository{
		getAPIKeyByHashFunc: func(ctx context.Context, hash string) (*APIKey, error) {
			if hash == keyHash {
				return &APIKey{
					ID:             "key-002",
					OrganizationID: "org-002",
					UserID:         sql.NullString{Valid: false},
				}, nil
			}
			return nil, nil
		},
	}

	handler := APIKeyAuthMiddleware(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeyID, ok := r.Context().Value(APIKeyIDKey).(string)
		if !ok || apiKeyID != "key-002" {
			t.Errorf("api_key_id = %q, want %q", apiKeyID, "key-002")
		}
		orgID, ok := GetOrgID(r.Context())
		if !ok || orgID != "org-002" {
			t.Errorf("org_id = %q, want %q", orgID, "org-002")
		}
		_, ok = GetUserID(r.Context())
		if ok {
			t.Error("user_id should not be set in context without user")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestComputeAPIKeyHash_Deterministic(t *testing.T) {
	t.Parallel()

	key := "my-api-key-for-testing"
	hash1 := ComputeAPIKeyHash(key)
	hash2 := ComputeAPIKeyHash(key)

	if hash1 == "" {
		t.Error("hash is empty")
	}
	if hash1 != hash2 {
		t.Error("computeAPIKeyHash should be deterministic")
	}
}

func TestContextHelpers_NoValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	if _, ok := GetUserID(ctx); ok {
		t.Error("GetUserID should return false for empty context")
	}
	if _, ok := GetOrgID(ctx); ok {
		t.Error("GetOrgID should return false for empty context")
	}
	if _, ok := GetRole(ctx); ok {
		t.Error("GetRole should return false for empty context")
	}
}
