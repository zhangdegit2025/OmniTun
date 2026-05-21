package auth

import "net/http"

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := GetRole(r.Context())
			if !ok {
				http.Error(w, `{"error":{"code":"unauthorized","message":"no role in context"}}`, http.StatusUnauthorized)
				return
			}
			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":{"code":"forbidden","message":"insufficient permissions"}}`, http.StatusForbidden)
		})
	}
}
