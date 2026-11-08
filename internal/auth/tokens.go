package auth

import (
	"crypto/rand"
	"encoding/base64"
)

// RandomToken returns a URL-safe random token of n bytes (base64-encoded).
func RandomToken(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is non-recoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashSessionID is the database key for a session token. Sessions are stored
// hashed for the same reason API tokens are: read access to the SQLite file
// must not yield a working login.
func HashSessionID(token string) string { return HashAPIToken(token) }
