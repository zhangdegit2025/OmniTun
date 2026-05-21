package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserIDKey  contextKey = "user_id"
	OrgIDKey   contextKey = "org_id"
	RoleKey    contextKey = "role"
	APIKeyIDKey contextKey = "api_key_id"
)

func JWTAuthMiddleware(jwtMgr *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			claims, err := jwtMgr.ValidateAccessToken(parts[1])
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid token: %v", err), http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.Subject)
			ctx = context.WithValue(ctx, OrgIDKey, claims.OrgID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func APIKeyAuthMiddleware(repo Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				http.Error(w, "missing API key header", http.StatusUnauthorized)
				return
			}

			keyHash := ComputeAPIKeyHash(apiKey)
			key, err := repo.GetAPIKeyByHash(r.Context(), keyHash)
			if err != nil {
				http.Error(w, "invalid API key", http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, APIKeyIDKey, key.ID)
			if key.UserID.Valid {
				ctx = context.WithValue(ctx, UserIDKey, key.UserID.String)
			}
			ctx = context.WithValue(ctx, OrgIDKey, key.OrganizationID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ComputeAPIKeyHash(key string) string {
	mac := hmac.New(sha256.New, []byte("omnitun-api-key-hmac-key"))
	mac.Write([]byte(key))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDKey).(string)
	return id, ok
}

func GetOrgID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(OrgIDKey).(string)
	return id, ok
}

func GetRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleKey).(string)
	return role, ok
}
