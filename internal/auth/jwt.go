package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/omnitun/omnitun/pkg/config"
)

type Claims struct {
	jwt.RegisteredClaims
	OrgID string `json:"org_id"`
	Role  string `json:"role"`
}

type JWTManager struct {
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	jwtSecret     []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	logger        *slog.Logger
}

func NewJWTManager(cfg config.AuthConfig) (*JWTManager, error) {
	logger := slog.Default().With("component", "jwt_manager")

	accessExpiry := time.Duration(cfg.TokenExpiry) * time.Second
	if accessExpiry <= 0 {
		accessExpiry = 1 * time.Hour
	}
	refreshExpiry := 30 * 24 * time.Hour

	mgr := &JWTManager{
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
		logger:        logger,
	}

	if cfg.JWTSecret != "" {
		mgr.jwtSecret = []byte(cfg.JWTSecret)
	}

	privateKey, publicKey, err := loadOrGenerateKeys(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load or generate RSA keys: %w", err)
	}
	mgr.privateKey = privateKey
	mgr.publicKey = publicKey

	return mgr, nil
}

func loadOrGenerateKeys(cfg config.AuthConfig) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	if cfg.JWTPrivateKey != "" && cfg.JWTPublicKey != "" {
		privateKey, err := parsePrivateKey([]byte(cfg.JWTPrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse JWT private key: %w", err)
		}
		publicKey, err := parsePublicKey([]byte(cfg.JWTPublicKey))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse JWT public key: %w", err)
		}
		slog.Info("loaded JWT RSA key pair from config")
		return privateKey, publicKey, nil
	}

	if cfg.JWTSecret != "" {
		slog.Info("using HS256 with JWT secret from config (RSA keys not provided)")
		return nil, nil, nil
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	})

	slog.Warn("generated new RSA key pair - save these keys for production use",
		"private_key", string(privateKeyPEM),
		"public_key", string(publicKeyPEM),
	)

	if err := os.WriteFile("jwt_rsa_private.pem", privateKeyPEM, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to write private key: %w", err)
	}
	if err := os.WriteFile("jwt_rsa_public.pem", publicKeyPEM, 0644); err != nil {
		return nil, nil, fmt.Errorf("failed to write public key: %w", err)
	}
	slog.Info("RSA key pair saved to jwt_rsa_private.pem and jwt_rsa_public.pem")

	return privateKey, &privateKey.PublicKey, nil
}

func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key as PKCS1 (%w) or PKCS8 (%w)", err, err2)
		}
		rsaKey, ok := pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("parsed key is not an RSA private key")
		}
		return rsaKey, nil
	}
	return key, nil
}

func parsePublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		key, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse public key as PKIX (%w) or PKCS1 (%w)", err, err2)
		}
		return key, nil
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("parsed key is not an RSA public key")
	}
	return rsaPub, nil
}

func (m *JWTManager) IssueAccessToken(userID, orgID, role string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        generateJTI(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
		},
		OrgID: orgID,
		Role:  role,
	}

	var token *jwt.Token
	var signingKey interface{}

	if m.privateKey != nil {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		signingKey = m.privateKey
	} else if len(m.jwtSecret) > 0 {
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signingKey = m.jwtSecret
	} else {
		return "", fmt.Errorf("no signing key available")
	}

	return token.SignedString(signingKey)
}

func (m *JWTManager) IssueRefreshToken(userID string) (string, error) {
	token, err := GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	m.logger.Debug("issued refresh token", "user_id", userID)
	return token, nil
}

func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	var keyFunc jwt.Keyfunc
	if m.publicKey != nil {
		keyFunc = func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return m.publicKey, nil
		}
	} else if len(m.jwtSecret) > 0 {
		keyFunc = func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return m.jwtSecret, nil
		}
	} else {
		return nil, fmt.Errorf("no public key or secret available for JWT validation")
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func generateJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
