package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	pw := "correct-horse-battery-staple"
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("unexpected hash prefix: %q", h)
	}
	if err := VerifyPassword(pw, h); err != nil {
		t.Fatalf("verify same password failed: %v", err)
	}
	if err := VerifyPassword("wrong", h); err == nil {
		t.Fatal("verify with wrong password should fail")
	}
}

func TestHashPassword_RejectsEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("expected error on empty password")
	}
}

func TestHashPassword_UniqueSaltPerCall(t *testing.T) {
	pw := "same-input"
	h1, _ := HashPassword(pw)
	h2, _ := HashPassword(pw)
	if h1 == h2 {
		t.Fatal("two hashes of the same password collided — salt is not random")
	}
}

func TestVerifyPassword_BadFormat(t *testing.T) {
	cases := []string{
		"",
		"not-a-hash",
		"$argon2id$",
		"$argon2id$v=19$m=64,t=2,p=2$badbase64$alsobadbase64",
		"$bcrypt$wrongalgo$...",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if err := VerifyPassword("any", c); err == nil {
				t.Fatalf("expected error on malformed hash %q", c)
			}
		})
	}
}

func TestRandomToken_Length(t *testing.T) {
	tok := RandomToken(32)
	// 32 raw bytes -> 43 base64url chars (no padding)
	if len(tok) != 43 {
		t.Fatalf("expected 43 chars for 32 raw bytes, got %d (%q)", len(tok), tok)
	}
	if tok == RandomToken(32) {
		t.Fatal("two consecutive RandomToken calls returned the same value")
	}
}
