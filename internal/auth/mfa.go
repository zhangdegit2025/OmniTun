package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

func GenerateTOTPSecret(email string) (string, string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate TOTP secret: %w", err)
	}
	secret := strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "=")

	issuer := "OmniTun"
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", "6")
	params.Set("period", "30")

	qrURL := fmt.Sprintf("otpauth://totp/%s:%s?%s",
		url.PathEscape(issuer),
		url.PathEscape(email),
		params.Encode(),
	)

	return secret, qrURL, nil
}

func ValidateTOTPCode(secret, code string) bool {
	if len(code) != 6 {
		return false
	}
	return totpCode(secret, time.Now()) == code
}

func totpCode(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}

	counter := uint64(t.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0xf
	binary := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	otp := binary % 1000000

	return fmt.Sprintf("%06d", otp)
}

func init() {
	_ = math.MaxInt32
}
