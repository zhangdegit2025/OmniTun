package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func SuperAdminMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractAdminToken(r)
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
			claims, err := validateSuperAdminToken(token, secret)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, 401)
				return
			}
			ctx := context.WithValue(r.Context(), contextKey("admin_id"), claims["sub"])
			ctx = context.WithValue(ctx, contextKey("admin_role"), claims["role"])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type SuperAdminClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

func IssueSuperAdminToken(adminID, role, secret string) (string, error) {
	claims := &SuperAdminClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: adminID,
		},
		Role: role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func validateSuperAdminToken(tokenString, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return claims, nil
}

func GetAdminID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey("admin_id")).(string)
	return id, ok
}

func GetAdminRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(contextKey("admin_role")).(string)
	return role, ok
}

func extractAdminToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
