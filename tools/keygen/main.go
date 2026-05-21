//go:build ignore
// +build ignore

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keygen: %v\n", err)
		os.Exit(1)
	}
	pub := &key.PublicKey

	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal private: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("deploy/keys/jwt_rsa_private.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "write private: %v\n", err)
		os.Exit(1)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal public: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("deploy/keys/jwt_rsa_public.pem", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write public: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("JWT RSA key pair generated in deploy/keys/")
}
