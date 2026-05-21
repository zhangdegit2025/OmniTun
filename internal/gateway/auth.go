package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/omnitun/omnitun/pkg/errors"
)

type KeyValidator interface {
	ValidateAPIKey(ctx context.Context, key string) (agentID string, orgID string, err error)
}

type AuthManager struct {
	jwtSecret    []byte
	keyValidator KeyValidator
}

func NewAuthManager(jwtSecret string, keyValidator KeyValidator) *AuthManager {
	return &AuthManager{
		jwtSecret:    []byte(jwtSecret),
		keyValidator: keyValidator,
	}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Sub     string `json:"sub"`
	Org     string `json:"org"`
	Iat     int64  `json:"iat"`
	Exp     int64  `json:"exp"`
	AgentID string `json:"agent_id"`
}

func (a *AuthManager) AuthenticateAgent(ctx context.Context, token string) (agentID string, orgID string, err error) {
	if token == "" {
		return "", "", apperrors.Unauthorized("missing token")
	}

	if strings.HasPrefix(token, "ak_") {
		return a.validateAPIKey(ctx, token)
	}

	return a.validateJWT(token)
}

func (a *AuthManager) validateJWT(token string) (agentID string, orgID string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", apperrors.Unauthorized("invalid JWT format")
	}

	signingInput := parts[0] + "." + parts[1]

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", apperrors.Unauthorized("invalid JWT signature encoding")
	}

	mac := hmac.New(sha256.New, a.jwtSecret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expectedSig) {
		return "", "", apperrors.Unauthorized("invalid JWT signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", apperrors.Unauthorized("invalid JWT claims encoding")
	}

	var claims jwtClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", "", apperrors.Unauthorized("invalid JWT claims")
	}

	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return "", "", apperrors.Unauthorized("JWT token expired")
	}

	agentID = claims.AgentID
	if agentID == "" {
		agentID = claims.Sub
	}
	orgID = claims.Org

	if agentID == "" {
		return "", "", apperrors.Unauthorized("missing agent_id in JWT claims")
	}

	return agentID, orgID, nil
}

func (a *AuthManager) validateAPIKey(ctx context.Context, key string) (agentID string, orgID string, err error) {
	if a.keyValidator == nil {
		return "", "", apperrors.Unauthorized("API key validation not configured")
	}

	agentID, orgID, err = a.keyValidator.ValidateAPIKey(ctx, key)
	if err != nil {
		return "", "", apperrors.Unauthorized(fmt.Sprintf("invalid API key: %v", err))
	}

	return agentID, orgID, nil
}

type HelloAuthInfo struct {
	AgentID  string
	OrgID    string
	Version  string
	Hostname string
	OSType   string
}

func (a *AuthManager) generateJWT(claims jwtClaims) string {
	headerJSON, _ := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	mac := hmac.New(sha256.New, a.jwtSecret)
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig
}

func (a *AuthManager) AuthenticateHello(ctx context.Context, msg *WSMessage) (*HelloAuthInfo, error) {
	var hello HelloPayload
	if err := msg.UnmarshalPayload(&hello); err != nil {
		return nil, apperrors.BadRequest("invalid hello payload")
	}

	agentID, orgID, err := a.AuthenticateAgent(ctx, hello.Token)
	if err != nil {
		return nil, err
	}

	if hello.AgentID != "" && hello.AgentID != agentID {
		return nil, apperrors.Unauthorized("agent_id mismatch")
	}

	return &HelloAuthInfo{
		AgentID:  agentID,
		OrgID:    orgID,
		Version:  hello.Version,
		Hostname: hello.Hostname,
		OSType:   hello.OSType,
	}, nil
}
