// Package storetest provides a `Store` factory for unit tests across the
// codebase. Each call returns a fresh, ephemeral SQLite database in the test's
// own t.TempDir so tests stay isolated and parallel-safe.
package storetest

import (
	"testing"

	"github.com/Satan1an/webtermin/internal/store"
)

// New opens a fresh Store in a per-test temp directory and registers cleanup.
// Use it like:
//
//	st := storetest.New(t)
//	user, err := st.CreateUser(...)
func New(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("storetest.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
