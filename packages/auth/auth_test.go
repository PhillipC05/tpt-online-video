package auth

import (
	"strings"
	"testing"
)

// --- PasswordHasher ---

func TestPasswordHasher_HashAndCompare(t *testing.T) {
	h := NewPasswordHasher()
	password := "correct-horse-battery-staple"

	encoded, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Errorf("encoded hash should start with $argon2id$, got %q", encoded)
	}

	if !h.Compare(password, encoded) {
		t.Error("Compare: expected true for correct password, got false")
	}
}

func TestPasswordHasher_WrongPassword(t *testing.T) {
	h := NewPasswordHasher()
	encoded, err := h.Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if h.Compare("wrong", encoded) {
		t.Error("Compare: expected false for wrong password, got true")
	}
}

func TestPasswordHasher_UniqueHashes(t *testing.T) {
	h := NewPasswordHasher()
	password := "same-password"

	hash1, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash 1: %v", err)
	}
	hash2, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash 2: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected two hashes of the same password to differ (different salts)")
	}

	// Both should still verify correctly.
	if !h.Compare(password, hash1) {
		t.Error("Compare hash1: expected true")
	}
	if !h.Compare(password, hash2) {
		t.Error("Compare hash2: expected true")
	}
}

func TestPasswordHasher_EmptyPassword(t *testing.T) {
	h := NewPasswordHasher()
	encoded, err := h.Hash("")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !h.Compare("", encoded) {
		t.Error("Compare: expected true for empty password")
	}
	if h.Compare("not-empty", encoded) {
		t.Error("Compare: expected false for non-empty password against empty hash")
	}
}

func TestPasswordHasher_Compare_InvalidHash(t *testing.T) {
	h := NewPasswordHasher()
	// Malformed encoded strings should return false, not panic.
	cases := []string{
		"",
		"notahash",
		"$argon2id$v=19$m=65536,t=3,p=2$badsalt",
		"$argon2id$v=19$m=65536,t=3,p=2$salt$hash$extrafield",
	}
	for _, c := range cases {
		if h.Compare("password", c) {
			t.Errorf("Compare(%q): expected false for malformed hash", c)
		}
	}
}

// --- TokenManager ---

func TestTokenManager_NewRandomToken(t *testing.T) {
	m := NewTokenManager()
	plain, hashed, err := m.NewRandomToken()
	if err != nil {
		t.Fatalf("NewRandomToken: %v", err)
	}

	if len(plain) == 0 {
		t.Error("expected non-empty plaintext token")
	}
	if len(hashed) == 0 {
		t.Error("expected non-empty hashed token")
	}
	if plain == hashed {
		t.Error("plaintext and hashed token should differ")
	}
}

func TestTokenManager_NewRandomToken_Unique(t *testing.T) {
	m := NewTokenManager()
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		plain, _, err := m.NewRandomToken()
		if err != nil {
			t.Fatalf("NewRandomToken: %v", err)
		}
		if seen[plain] {
			t.Fatalf("duplicate token generated: %q", plain)
		}
		seen[plain] = true
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	token := "abc123"
	h1 := HashToken(token)
	h2 := HashToken(token)
	if h1 != h2 {
		t.Errorf("HashToken should be deterministic: got %q and %q", h1, h2)
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	if HashToken("a") == HashToken("b") {
		t.Error("expected different hashes for different inputs")
	}
}

func TestHashToken_MatchesRandomToken(t *testing.T) {
	m := NewTokenManager()
	plain, hashed, err := m.NewRandomToken()
	if err != nil {
		t.Fatalf("NewRandomToken: %v", err)
	}
	if HashToken(plain) != hashed {
		t.Error("HashToken(plain) should equal the hashed token from NewRandomToken")
	}
}

// --- parseHash ---

func TestParseHash_InvalidFormats(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too few parts", "$argon2id$v=19$m=65536"},
		{"wrong algorithm", "$argon2i$v=19$m=65536,t=3,p=2$salt$hash"},
		{"wrong version", "$argon2id$v=18$m=65536,t=3,p=2$salt$hash"},
		{"bad memory param", "$argon2id$v=19$m=x,t=3,p=2$salt$hash"},
		{"bad iterations param", "$argon2id$v=19$m=65536,t=x,p=2$salt$hash"},
		{"bad parallelism param", "$argon2id$v=19$m=65536,t=3,p=x$salt$hash"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseHash(c.input)
			if err == nil {
				t.Errorf("expected error for %q, got nil", c.input)
			}
		})
	}
}

// --- NewEmailSender factory ---

func TestNewEmailSender_LogProvider(t *testing.T) {
	cfg := EmailConfig{Provider: "log"}
	sender := NewEmailSender(cfg)
	if sender == nil {
		t.Fatal("expected non-nil EmailSender")
	}
	// logSender should satisfy the interface and not error.
	ctx := t.Context()
	if err := sender.SendWelcomeEmail(ctx, "test@example.com", "Alice"); err != nil {
		t.Errorf("SendWelcomeEmail: %v", err)
	}
	if err := sender.SendPasswordResetEmail(ctx, "test@example.com", "https://example.com/reset"); err != nil {
		t.Errorf("SendPasswordResetEmail: %v", err)
	}
	if err := sender.SendEmailVerification(ctx, "test@example.com", "https://example.com/verify"); err != nil {
		t.Errorf("SendEmailVerification: %v", err)
	}
}

func TestNewEmailSender_EmptyProviderDefaultsToLog(t *testing.T) {
	cfg := EmailConfig{Provider: ""}
	sender := NewEmailSender(cfg)
	if sender == nil {
		t.Fatal("expected non-nil EmailSender for empty provider")
	}
}

func TestNewEmailSender_UnknownProviderDefaultsToLog(t *testing.T) {
	cfg := EmailConfig{Provider: "unknown-provider"}
	sender := NewEmailSender(cfg)
	if sender == nil {
		t.Fatal("expected non-nil EmailSender for unknown provider")
	}
}
