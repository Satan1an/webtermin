package users

import (
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	good := []string{"alice", "bob", "_root", "user-1", "u_2", "a", strings.Repeat("a", 32)}
	for _, n := range good {
		if !ValidName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}
	bad := []string{
		"",                       // empty
		"1bob",                   // starts with digit
		"-bob",                   // starts with dash
		"Bob",                    // uppercase
		"bob bob",                // space
		"bob;rm",                 // shell metachars
		"bob$",                   // dollar
		"bob\nrm",                // newline
		"bob/etc",                // slash
		strings.Repeat("a", 33),  // too long (max 32)
		"../../etc",              // traversal
		"root\x00admin",          // null byte
	}
	for _, n := range bad {
		if ValidName(n) {
			t.Errorf("expected %q to be REJECTED, but it passed", n)
		}
	}
}

func TestParseKeyLine_Valid(t *testing.T) {
	// ed25519 fixture (real-shape but throwaway key).
	line := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINaW5w2QwQqlPckLh6e1J0G7+1BcoQ+wHwgWA0aL3rJI demo@example.com"
	k, ok := parseKeyLine(line)
	if !ok {
		t.Fatal("parseKeyLine returned !ok on a well-formed key")
	}
	if k.Type != "ssh-ed25519" {
		t.Errorf("type: got %q, want ssh-ed25519", k.Type)
	}
	if !strings.HasPrefix(k.Fingerprint, "SHA256:") {
		t.Errorf("fingerprint: got %q, want SHA256:... prefix", k.Fingerprint)
	}
	if k.Comment != "demo@example.com" {
		t.Errorf("comment: got %q", k.Comment)
	}
	if k.Raw != line {
		t.Errorf("raw not preserved: %q", k.Raw)
	}
}

func TestParseKeyLine_RejectsGarbage(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"ssh-rsa",                  // missing blob
		"not-base64",               // single token
		"ssh-rsa !!!notb64!!! me",  // invalid base64
	}
	for _, line := range bad {
		if _, ok := parseKeyLine(line); ok {
			t.Errorf("expected garbage %q to be rejected", line)
		}
	}
}

func TestParseKeys_SkipsCommentsAndBlanks(t *testing.T) {
	src := `# this is a comment

ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINaW5w2QwQqlPckLh6e1J0G7+1BcoQ+wHwgWA0aL3rJI alice
# another comment
not-a-key-line
`
	keys := parseKeys(strings.NewReader(src))
	if len(keys) != 1 {
		t.Fatalf("expected 1 parsed key, got %d", len(keys))
	}
	if keys[0].Comment != "alice" {
		t.Fatalf("comment: got %q", keys[0].Comment)
	}
}

func TestValidPath(t *testing.T) {
	good := []string{"/home/alice", "/var/lib/data"}
	bad := []string{"", "home/alice", "/home/../etc"}
	for _, p := range good {
		if !validPath(p) {
			t.Errorf("expected %q to be valid path", p)
		}
	}
	for _, p := range bad {
		if validPath(p) {
			t.Errorf("expected %q to be rejected", p)
		}
	}
}
