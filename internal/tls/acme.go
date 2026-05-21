package tls

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

const (
	defaultDataDir      = "data"
	acmeSubDir          = "acme"
	certsSubDir         = "certs"
	accountKeyFile      = "account.key"
	renewalWindow       = 30 * 24 * time.Hour
	defaultCertValidity = 365 * 24 * time.Hour
)

type Manager struct {
	cfg          *config.Config
	logger       *slog.Logger
	mu           sync.RWMutex
	certificates map[string]*CertEntry
	privKey      *rsa.PrivateKey
	dataDir      string
}

type CertEntry struct {
	Domain    string
	CertPath  string
	KeyPath   string
	NotAfter  time.Time
	AutoRenew bool
	Renewing  bool
}

func NewManager(cfg *config.Config, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	dataDir := defaultDataDir
	if err := os.MkdirAll(filepath.Join(dataDir, acmeSubDir), 0700); err != nil {
		return nil, fmt.Errorf("create acme dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, certsSubDir), 0700); err != nil {
		return nil, fmt.Errorf("create certs dir: %w", err)
	}

	keyPath := filepath.Join(dataDir, acmeSubDir, accountKeyFile)
	privKey, err := loadOrGenerateKey(keyPath, logger)
	if err != nil {
		return nil, fmt.Errorf("load or generate account key: %w", err)
	}

	return &Manager{
		cfg:          cfg,
		logger:       logger,
		certificates: make(map[string]*CertEntry),
		privKey:      privKey,
		dataDir:      dataDir,
	}, nil
}

func (m *Manager) SelfSignedCert(domain string) (*CertEntry, error) {
	m.mu.RLock()
	if entry, ok := m.certificates[domain]; ok {
		m.mu.RUnlock()
		return entry, nil
	}
	m.mu.RUnlock()

	return m.generateAndStoreCert(domain)
}

func (m *Manager) generateAndStoreCert(domain string) (*CertEntry, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ECDSA key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"OmniTun"},
			CommonName:   domain,
		},
		NotBefore: now.Add(-1 * time.Hour),
		NotAfter:  now.Add(defaultCertValidity),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, err := m.storeCertificateLocked(domain, certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	m.logger.Info("self-signed certificate generated", "domain", domain, "notAfter", entry.NotAfter)
	return entry, nil
}

func (m *Manager) ObtainCertificate(ctx context.Context, domain string) (*CertEntry, error) {
	// TODO: Replace with go-acme/lego when network available
	// import "github.com/go-acme/lego/v4/..."
	// client, err := lego.NewClient(lego.NewConfig(m.privKey))
	// provider := httpchallenge.NewProviderServer("", fmt.Sprintf(":%d", m.cfg.ServerPort))
	// client.Challenge.SetHTTP01Provider(provider)
	// cert, err := client.Certificate.Obtain(certificate.ObtainRequest{Domain: domain})
	// ...
	return nil, fmt.Errorf("ACME client not available: lego dependency not yet integrated")
}

func (m *Manager) RenewCertificate(ctx context.Context, domain string) error {
	m.mu.Lock()
	entry, ok := m.certificates[domain]
	if ok && entry.Renewing {
		m.mu.Unlock()
		return fmt.Errorf("certificate for %s is already being renewed", domain)
	}
	if ok {
		entry.Renewing = true
	}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		if e, ok := m.certificates[domain]; ok {
			e.Renewing = false
		}
		m.mu.Unlock()
	}()

	m.logger.Info("renewing certificate", "domain", domain)

	newEntry, err := m.generateAndStoreCert(domain)
	if err != nil {
		return fmt.Errorf("renew self-signed cert for %s: %w", domain, err)
	}

	m.mu.Lock()
	m.certificates[domain] = newEntry
	m.mu.Unlock()

	m.logger.Info("certificate renewed", "domain", domain, "notAfter", newEntry.NotAfter)
	return nil
}

func (m *Manager) GetCertificate(domain string) (*CertEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.certificates[domain]
	return entry, ok
}

func (m *Manager) IsExpiringSoon(domain string) (bool, error) {
	m.mu.RLock()
	entry, ok := m.certificates[domain]
	m.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("no certificate found for domain %s", domain)
	}

	return time.Until(entry.NotAfter) <= renewalWindow, nil
}

func (m *Manager) StartAutoRenewal(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	m.logger.Info("auto-renewal background task started")

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("auto-renewal stopped")
			return
		case <-ticker.C:
			m.checkAndRenew(ctx)
		}
	}
}

func (m *Manager) checkAndRenew(ctx context.Context) {
	m.mu.RLock()
	domains := make([]string, 0, len(m.certificates))
	for domain, entry := range m.certificates {
		if entry.AutoRenew && time.Until(entry.NotAfter) < renewalWindow {
			domains = append(domains, domain)
		}
	}
	m.mu.RUnlock()

	for _, domain := range domains {
		if err := m.RenewCertificate(ctx, domain); err != nil {
			m.logger.Error("auto-renewal failed", "domain", domain, "error", err)
		}
	}
}

func (m *Manager) GetWildcardCertificate(ctx context.Context, domain string) (*CertEntry, error) {
	// TODO: Replace with go-acme/lego when network available
	// Wildcard certificates require DNS-01 challenge with lego
	// client.Challenge.SetDNS01Provider(...)
	return nil, fmt.Errorf("ACME wildcard not available: lego dependency not yet integrated")
}

func (m *Manager) UploadCustomCertificate(domain string, certPEM, keyPEM []byte) (*CertEntry, error) {
	if _, _, err := parseAndValidateCert(certPEM, keyPEM); err != nil {
		return nil, fmt.Errorf("invalid certificate: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, err := m.storeCertificateLocked(domain, certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	m.logger.Info("custom certificate uploaded", "domain", domain, "notAfter", entry.NotAfter)
	return entry, nil
}

func (m *Manager) storeCertificateLocked(domain string, certPEM, keyPEM []byte) (*CertEntry, error) {
	dir := filepath.Join(m.dataDir, certsSubDir, domain)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cert dir for %s: %w", domain, err)
	}

	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		return nil, fmt.Errorf("write cert file: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("write key file: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	entry := &CertEntry{
		Domain:    domain,
		CertPath:  certPath,
		KeyPath:   keyPath,
		NotAfter:  cert.NotAfter,
		AutoRenew: true,
		Renewing:  false,
	}

	m.certificates[domain] = entry
	return entry, nil
}

func loadOrGenerateKey(keyPath string, logger *slog.Logger) (*rsa.PrivateKey, error) {
	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("failed to decode account key PEM")
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			parsedKey, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				return nil, fmt.Errorf("parse RSA private key: %w (pkcs8: %w)", err, err2)
			}
			rsaKey, ok := parsedKey.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("account key is not RSA private key")
			}
			return rsaKey, nil
		}
		return key, nil
	}

	logger.Info("generating new ACME account key")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("create key directory: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("write account key: %w", err)
	}

	return key, nil
}

func parseAndValidateCert(certPEM, keyPEM []byte) (*x509.Certificate, crypto.PrivateKey, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode key PEM")
	}

	var privKey crypto.PrivateKey
	privKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		privKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			privKey, err = x509.ParseECPrivateKey(keyBlock.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("parse private key: %w", err)
			}
		}
	}

	return cert, privKey, nil
}
