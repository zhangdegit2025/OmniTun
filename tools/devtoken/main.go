//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	claims := jwt.MapClaims{
		"sub":    "00000000-0000-0000-0000-000000000001",
		"org_id": "00000000-0000-0000-0000-000000000001",
		"role":   "owner",
		"jti":    fmt.Sprintf("%d", time.Now().UnixNano()),
		"iat":    time.Now().Unix(),
		"exp":    time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("change-me-in-production"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(tokenString)
}
