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
