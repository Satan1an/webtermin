package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// APITokenPrefix marks a webtermin-issued programmatic access token. The full
// format is "wt_" + 43 base64-url chars (32 raw bytes), giving 256 bits of
// entropy. Tokens are stored as a SHA-256 hex digest — never in plaintext.
const APITokenPrefix = "wt_"

// NewAPIToken returns a freshly generated (plaintext, hash) pair.
// The plaintext is shown to the user exactly once at creation time.
func NewAPIToken() (plaintext, hash string) {
	plaintext = APITokenPrefix + RandomToken(32)
	return plaintext, HashAPIToken(plaintext)
}

// HashAPIToken returns the storage form of an API token. We use SHA-256 (not
// Argon2id) because tokens are 256-bit uniform-random and don't need slow
// hashing — they aren't brute-forceable in any practical sense.
func HashAPIToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// LooksLikeAPIToken returns true if s has the webtermin token shape. Cheap
// preflight check before we hit the DB.
func LooksLikeAPIToken(s string) bool {
	return strings.HasPrefix(s, APITokenPrefix) && len(s) > len(APITokenPrefix)+10
}
