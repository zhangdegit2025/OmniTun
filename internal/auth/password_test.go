package auth

import (
	"testing"
)

func TestValidatePassword_Strong_Passes(t *testing.T) {
	t.Parallel()

	if err := ValidatePassword("Str0ng!Pass"); err != nil {
		t.Errorf("ValidatePassword(Str0ng!Pass) should pass, got: %v", err)
	}
}

func TestValidatePassword_TooShort_Fails(t *testing.T) {
	t.Parallel()

	err := ValidatePassword("Abc1")
	if err != ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestValidatePassword_NoUppercase_Fails(t *testing.T) {
	t.Parallel()

	err := ValidatePassword("password1")
	if err != ErrPasswordMissingUpper {
		t.Errorf("expected ErrPasswordMissingUpper, got %v", err)
	}
}

func TestValidatePassword_NoLowercase_Fails(t *testing.T) {
	t.Parallel()

	err := ValidatePassword("PASSWORD1")
	if err != ErrPasswordMissingLower {
		t.Errorf("expected ErrPasswordMissingLower, got %v", err)
	}
}

func TestValidatePassword_NoDigit_Fails(t *testing.T) {
	t.Parallel()

	err := ValidatePassword("Password")
	if err != ErrPasswordMissingDigit {
		t.Errorf("expected ErrPasswordMissingDigit, got %v", err)
	}
}

func TestHashPassword_Success_Matches(t *testing.T) {
	t.Parallel()

	password := "ValidP@ssw0rd"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("hash is empty")
	}
	if hash == password {
		t.Fatal("hash should not equal password")
	}

	if err := CheckPassword(hash, password); err != nil {
		t.Errorf("CheckPassword should succeed: %v", err)
	}
}

func TestCheckPassword_Wrong_Fails(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("CorrectP@ss1")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if err := CheckPassword(hash, "WrongP@ss1"); err == nil {
		t.Error("CheckPassword should fail for wrong password")
	}
}

func TestHashPassword_Uniqueness_DifferentSalts(t *testing.T) {
	t.Parallel()

	password := "SameP@ssw0rd"
	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("first HashPassword failed: %v", err)
	}
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("second HashPassword failed: %v", err)
	}

	if hash1 == hash2 {
		t.Error("same password hashed twice should produce different hashes due to salt")
	}

	if err := CheckPassword(hash1, password); err != nil {
		t.Errorf("hash1 should verify: %v", err)
	}
	if err := CheckPassword(hash2, password); err != nil {
		t.Errorf("hash2 should verify: %v", err)
	}
}
