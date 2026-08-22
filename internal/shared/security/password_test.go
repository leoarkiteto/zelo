package security

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	h := NewPasswordHasher("test-pepper")
	hash, err := h.Hash("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	ok, err := h.Verify(hash, "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !ok {
		t.Fatal("Verify() = false, want true for correct password")
	}
	ok, err = h.Verify(hash, "wrong-password")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if ok {
		t.Fatal("Verify() = true, want false for wrong password")
	}
}

func TestPasswordPolicy(t *testing.T) {
	h := NewPasswordHasher("test-pepper")
	if err := h.ValidatePassword("short"); err == nil {
		t.Fatal("ValidatePassword() = nil, want error for short password")
	}
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if err := h.ValidatePassword(string(long)); err == nil {
		t.Fatal("ValidatePassword() = nil, want error for long password")
	}
	if err := h.ValidatePassword("a-valid-password-123"); err != nil {
		t.Fatalf("ValidatePassword() error = %v", err)
	}
}

func TestPasswordVerifyRejectsMalformedHash(t *testing.T) {
	h := NewPasswordHasher("test-pepper")
	if _, err := h.Verify("not-a-hash", "password"); err == nil {
		t.Fatal("Verify() = nil error, want error for malformed hash")
	}
}

func TestPasswordHashIsPHCArgon2id(t *testing.T) {
	h := NewPasswordHasher("test-pepper")
	hash, err := h.Hash("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		t.Fatalf("hash = %q, want PHC-formatted $argon2id$ string", hash)
	}
}

func TestPasswordHashBindsToPepper(t *testing.T) {
	password := "correct-horse-battery-staple"
	h1 := NewPasswordHasher("pepper-a")
	h2 := NewPasswordHasher("pepper-b")
	hash, err := h1.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	// The same password hashed with a different pepper must not verify.
	if ok, err := h2.Verify(hash, password); err != nil || ok {
		t.Fatalf("Verify() with wrong pepper = %v, %v; want false", ok, err)
	}
	// And the original pepper still verifies it.
	if ok, err := h1.Verify(hash, password); err != nil || !ok {
		t.Fatalf("Verify() with original pepper = %v, %v; want true", ok, err)
	}
}
