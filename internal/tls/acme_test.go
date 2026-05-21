package tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/omnitun/omnitun/pkg/config"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		ServerPort: 8443,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m := &Manager{
		cfg:          cfg,
		logger:       logger,
		certificates: make(map[string]*CertEntry),
		dataDir:      tmpDir,
	}

	acmeDir := filepath.Join(tmpDir, acmeSubDir)
	certsDir := filepath.Join(tmpDir, certsSubDir)
	if err := os.MkdirAll(acmeDir, 0700); err != nil {
		t.Fatalf("create acme dir: %v", err)
	}
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		t.Fatalf("create certs dir: %v", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	m.privKey = key

	return m
}

func generateTestCert(t *testing.T, domain string, notAfter time.Time) (*CertEntry, *ecdsa.PrivateKey) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"OmniTun Test"},
			CommonName:   domain,
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	_ = derBytes

	entry := &CertEntry{
		Domain:    domain,
		CertPath:  "",
		KeyPath:   "",
		NotAfter:  notAfter,
		AutoRenew: true,
		Renewing:  false,
	}

	return entry, priv
}

func TestManager_SelfSignedCertificate(t *testing.T) {
	m := newTestManager(t)

	entry, err := m.SelfSignedCert("localhost")
	if err != nil {
		t.Fatalf("SelfSignedCert failed: %v", err)
	}

	if entry.Domain != "localhost" {
		t.Errorf("expected domain localhost, got %s", entry.Domain)
	}

	if entry.CertPath == "" || entry.KeyPath == "" {
		t.Error("expected non-empty CertPath and KeyPath")
	}

	certData, err := os.ReadFile(entry.CertPath)
	if err != nil {
		t.Fatalf("read cert file: %v", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		t.Fatal("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	expectedExpiry := time.Now().Add(defaultCertValidity)
	diff := cert.NotAfter.Sub(expectedExpiry).Abs()
	if diff > time.Hour {
		t.Errorf("certificate expiry too far from expected: got %v, expected ~%v (diff %v)", cert.NotAfter, expectedExpiry, diff)
	}

	if !cert.IsCA {
		if cert.BasicConstraintsValid {
			// Self-signed cert may or may not be CA; this is fine
		}
	}

	if entry.NotAfter.IsZero() {
		t.Error("expected non-zero NotAfter on CertEntry")
	}

	// Test cache hit
	entry2, err := m.SelfSignedCert("localhost")
	if err != nil {
		t.Fatalf("second SelfSignedCert failed: %v", err)
	}
	if entry2 != entry {
		t.Error("expected cached entry to be same pointer")
	}
}

func TestManager_CacheCertificate(t *testing.T) {
	m := newTestManager(t)

	entry1, _ := m.SelfSignedCert("example.com")

	retrieved, ok := m.GetCertificate("example.com")
	if !ok {
		t.Fatal("expected certificate to be found in cache")
	}

	if retrieved != entry1 {
		t.Error("expected cached entry to be same object")
	}

	if retrieved.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", retrieved.Domain)
	}

	_, ok = m.GetCertificate("nonexistent.com")
	if ok {
		t.Error("expected no certificate for nonexistent.com")
	}

	entry2, _ := m.SelfSignedCert("test.example.com")
	retrieved2, ok := m.GetCertificate("test.example.com")
	if !ok || retrieved2 != entry2 {
		t.Error("expected cached entry for test.example.com")
	}
}

func TestManager_AutoRenewCheck(t *testing.T) {
	m := newTestManager(t)

	expiringSoon := time.Now().Add(15 * 24 * time.Hour)
	entryExpiring, _ := generateTestCert(t, "expiring.example.com", expiringSoon)
	m.mu.Lock()
	m.certificates["expiring.example.com"] = entryExpiring
	m.mu.Unlock()

	isExpiring, err := m.IsExpiringSoon("expiring.example.com")
	if err != nil {
		t.Fatalf("IsExpiringSoon failed: %v", err)
	}
	if !isExpiring {
		t.Error("expected certificate with 15 days expiry to be expiring soon")
	}

	farFuture := time.Now().Add(60 * 24 * time.Hour)
	entryFar, _ := generateTestCert(t, "valid.example.com", farFuture)
	m.mu.Lock()
	m.certificates["valid.example.com"] = entryFar
	m.mu.Unlock()

	isExpiring, err = m.IsExpiringSoon("valid.example.com")
	if err != nil {
		t.Fatalf("IsExpiringSoon failed: %v", err)
	}
	if isExpiring {
		t.Error("expected certificate with 60 days expiry to NOT be expiring soon")
	}

	exactly30Days := time.Now().Add(30 * 24 * time.Hour)
	entry30, _ := generateTestCert(t, "thirty.example.com", exactly30Days)
	m.mu.Lock()
	m.certificates["thirty.example.com"] = entry30
	m.mu.Unlock()

	isExpiring, err = m.IsExpiringSoon("thirty.example.com")
	if err != nil {
		t.Fatalf("IsExpiringSoon failed: %v", err)
	}
	if !isExpiring {
		t.Error("expected certificate with exactly 30 days expiry to be expiring soon (within renewal window)")
	}

	_, err = m.IsExpiringSoon("nonexistent.example.com")
	if err == nil {
		t.Error("expected error for nonexistent domain")
	}
}

func TestManager_UploadCustomCertificate(t *testing.T) {
	m := newTestManager(t)

	// Generate fresh cert and key PEM for upload
	priv2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"OmniTun Custom"},
			CommonName:   "custom-upload.example.com",
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"custom-upload.example.com"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv2.PublicKey, priv2)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv2)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	uploaded, err := m.UploadCustomCertificate("custom-upload.example.com", certPEM, keyPEM)
	if err != nil {
		t.Fatalf("UploadCustomCertificate failed: %v", err)
	}

	if uploaded.Domain != "custom-upload.example.com" {
		t.Errorf("expected domain custom-upload.example.com, got %s", uploaded.Domain)
	}

	cached, ok := m.GetCertificate("custom-upload.example.com")
	if !ok {
		t.Fatal("expected uploaded cert to be in cache")
	}
	if cached != uploaded {
		t.Error("expected cached entry to be same object")
	}

	_, err = m.UploadCustomCertificate("bad.example.com", []byte("not pem"), keyPEM)
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestManager_NewManager(t *testing.T) {
	cfg := &config.Config{
		ServerPort: 8443,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(cfg, logger)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager.cfg != cfg {
		t.Error("expected config to be stored")
	}

	if manager.logger != logger {
		t.Error("expected logger to be stored")
	}

	if manager.privKey == nil {
		t.Error("expected private key to be generated")
	}

	if manager.dataDir != defaultDataDir {
		t.Errorf("expected dataDir=%s, got %s", defaultDataDir, manager.dataDir)
	}

	// Verify account key file was written
	keyPath := filepath.Join(defaultDataDir, acmeSubDir, accountKeyFile)
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("account key file not found: %v", err)
	}

	// Verify manager loads existing key on second creation
	manager2, err := NewManager(cfg, logger)
	if err != nil {
		t.Fatalf("NewManager (reload) failed: %v", err)
	}
	if manager2.privKey == nil {
		t.Error("expected private key to be loaded on second creation")
	}

	// Cleanup
	os.RemoveAll(defaultDataDir)
}

func TestManager_RenewCertificate(t *testing.T) {
	m := newTestManager(t)

	entry, err := m.SelfSignedCert("renew-test.example.com")
	if err != nil {
		t.Fatalf("SelfSignedCert failed: %v", err)
	}

	oldCertPath := entry.CertPath
	oldCertData, err := os.ReadFile(oldCertPath)
	if err != nil {
		t.Fatalf("read old cert: %v", err)
	}

	ctx := context.Background()
	if err := m.RenewCertificate(ctx, "renew-test.example.com"); err != nil {
		t.Fatalf("RenewCertificate failed: %v", err)
	}

	newEntry, ok := m.GetCertificate("renew-test.example.com")
	if !ok {
		t.Fatal("expected renewed cert to be in cache")
	}

	newCertData, err := os.ReadFile(newEntry.CertPath)
	if err != nil {
		t.Fatalf("read new cert: %v", err)
	}

	if string(oldCertData) == string(newCertData) {
		t.Error("expected renewed certificate to differ from original")
	}

	if _, err := os.Stat(newEntry.CertPath); err != nil {
		t.Errorf("renewed cert file not found: %v", err)
	}
}
