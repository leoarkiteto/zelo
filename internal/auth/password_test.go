package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	h := PasswordHasher{}
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
	h := PasswordHasher{}
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
	h := PasswordHasher{}
	if _, err := h.Verify("not-a-hash", "password"); err == nil {
		t.Fatal("Verify() = nil error, want error for malformed hash")
	}
}
