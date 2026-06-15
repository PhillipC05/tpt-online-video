package live

import (
	"encoding/hex"
	"testing"
)

func TestGenerateStreamKey_Format(t *testing.T) {
	plain, hash, err := GenerateStreamKey()
	if err != nil {
		t.Fatalf("GenerateStreamKey: %v", err)
	}

	if len(plain) == 0 {
		t.Error("expected non-empty plaintext key")
	}
	if len(hash) == 0 {
		t.Error("expected non-empty hash")
	}

	// plaintext should be valid hex (32 random bytes → 64 hex chars)
	if _, err := hex.DecodeString(plain); err != nil {
		t.Errorf("plaintext key is not valid hex: %v", err)
	}
	if len(plain) != 64 {
		t.Errorf("expected 64-char hex plaintext, got %d", len(plain))
	}

	// hash should be valid hex (SHA-256 → 64 hex chars)
	if _, err := hex.DecodeString(hash); err != nil {
		t.Errorf("hash is not valid hex: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d", len(hash))
	}
}

func TestGenerateStreamKey_PlaintextAndHashDiffer(t *testing.T) {
	plain, hash, err := GenerateStreamKey()
	if err != nil {
		t.Fatalf("GenerateStreamKey: %v", err)
	}
	if plain == hash {
		t.Error("plaintext and hash should not be equal")
	}
}

func TestGenerateStreamKey_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		plain, _, err := GenerateStreamKey()
		if err != nil {
			t.Fatalf("GenerateStreamKey: %v", err)
		}
		if seen[plain] {
			t.Fatalf("duplicate stream key generated on iteration %d", i)
		}
		seen[plain] = true
	}
}

func TestHashStreamKey_Deterministic(t *testing.T) {
	key := "abc123"
	h1 := HashStreamKey(key)
	h2 := HashStreamKey(key)
	if h1 != h2 {
		t.Errorf("HashStreamKey should be deterministic: %q != %q", h1, h2)
	}
}

func TestHashStreamKey_DifferentInputs(t *testing.T) {
	if HashStreamKey("a") == HashStreamKey("b") {
		t.Error("different keys should produce different hashes")
	}
}

func TestHashStreamKey_MatchesGenerateHash(t *testing.T) {
	plain, hash, err := GenerateStreamKey()
	if err != nil {
		t.Fatalf("GenerateStreamKey: %v", err)
	}
	if HashStreamKey(plain) != hash {
		t.Error("HashStreamKey(plain) should equal the hash from GenerateStreamKey")
	}
}
