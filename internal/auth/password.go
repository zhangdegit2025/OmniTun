package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort        = errors.New("password must be at least 8 characters")
	ErrPasswordMissingUpper    = errors.New("password must contain at least one uppercase letter")
	ErrPasswordMissingLower    = errors.New("password must contain at least one lowercase letter")
	ErrPasswordMissingDigit    = errors.New("password must contain at least one digit")
)

func ValidatePassword(password string) error {
	if len([]rune(password)) < 8 {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return ErrPasswordMissingUpper
	}
	if !hasLower {
		return ErrPasswordMissingLower
	}
	if !hasDigit {
		return ErrPasswordMissingDigit
	}
	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
