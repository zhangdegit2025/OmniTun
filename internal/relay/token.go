package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type TokenValidator struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenValidator(secret string, ttl time.Duration) *TokenValidator {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return &TokenValidator{secret: []byte(secret), ttl: ttl}
}

// Generate creates a short-term tunnel token for the given tunnel_id.
func (v *TokenValidator) Generate(tunnelID string) (string, time.Time) {
	expires := time.Now().Add(v.ttl)
	payload := fmt.Sprintf("%s:%d", tunnelID, expires.Unix())
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig), expires
}

// Validate checks if the token is valid and returns the tunnel_id.
func (v *TokenValidator) Validate(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("empty token")
	}
	parts := splitLast(token, '.')
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token format")
	}
	payload, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid token signature")
	}

	idParts := splitN(payload, ':', 2)
	if len(idParts) != 2 {
		return "", fmt.Errorf("invalid payload format")
	}
	tunnelID := idParts[0]

	var expUnix int64
	fmt.Sscanf(idParts[1], "%d", &expUnix)
	if time.Now().Unix() > expUnix {
		return "", fmt.Errorf("token expired")
	}

	return tunnelID, nil
}

func splitLast(s string, sep byte) []string {
	i := len(s) - 1
	for i >= 0 && s[i] != sep {
		i--
	}
	if i < 0 {
		return []string{s}
	}
	return []string{s[:i], s[i+1:]}
}

func splitN(s string, sep byte, n int) []string {
	parts := make([]string, 0, n)
	start := 0
	count := 0
	for i := 0; i < len(s) && count < n-1; i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
			count++
		}
	}
	parts = append(parts, s[start:])
	return parts
}
