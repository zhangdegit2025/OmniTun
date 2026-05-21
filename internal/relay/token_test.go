package relay

import (
	"testing"
	"time"
)

func TestTokenValidator_GenerateAndValidate(t *testing.T) {
	v := NewTokenValidator("test-secret", 5*time.Minute)
	token, _ := v.Generate("tunnel-123")
	tid, err := v.Validate(token)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	if tid != "tunnel-123" {
		t.Errorf("tunnel_id = %q, want %q", tid, "tunnel-123")
	}
}

func TestTokenValidator_ExpiredToken(t *testing.T) {
	v := NewTokenValidator("test-secret", -1*time.Second)
	token, _ := v.Generate("tunnel-123")
	_, err := v.Validate(token)
	if err == nil {
		t.Fatal("expected expired error")
	}
}

func TestTokenValidator_InvalidSignature(t *testing.T) {
	v := NewTokenValidator("test-secret", 5*time.Minute)
	token, _ := v.Generate("tunnel-123")
	tampered := token + "x"
	_, err := v.Validate(tampered)
	if err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestTokenValidator_UniqueTokens(t *testing.T) {
	v := NewTokenValidator("test-secret", 5*time.Minute)
	t1, _ := v.Generate("tunnel-1")
	t2, _ := v.Generate("tunnel-2")
	if t1 == t2 {
		t.Fatal("tokens should be unique")
	}
	id1, _ := v.Validate(t1)
	id2, _ := v.Validate(t2)
	if id1 == id2 {
		t.Fatal("tunnel IDs should differ")
	}
}
