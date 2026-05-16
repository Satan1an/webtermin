package auth_test

import (
	"testing"
	"time"

	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/config"
	"github.com/Satan1an/webtermin/internal/store"
	"github.com/Satan1an/webtermin/internal/store/storetest"
)

func setupService(t *testing.T) (*auth.Service, *store.Store, *config.Config) {
	t.Helper()
	st := storetest.New(t)
	cfg := config.Default()
	cfg.Security.MaxLoginAttempts = 3
	cfg.Security.LockoutMin = 1
	cfg.Security.SessionTTLMin = 60
	return auth.New(st, cfg), st, cfg
}

func createUser(t *testing.T, st *store.Store, name, pw string) *store.User {
	t.Helper()
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := st.CreateUser(name, h, "", "admin", true)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestLogin_Success(t *testing.T) {
	svc, st, _ := setupService(t)
	createUser(t, st, "alice", "correct-horse-battery")

	res, err := svc.Login("alice", "correct-horse-battery", "", "1.2.3.4", "ua/1.0")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.User.Username != "alice" {
		t.Fatalf("wrong user: %s", res.User.Username)
	}
	if res.Session.CSRFToken == "" {
		t.Fatal("no CSRF token issued")
	}
	if time.Until(res.Session.ExpiresAt) <= 0 {
		t.Fatal("session already expired at creation")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, st, _ := setupService(t)
	createUser(t, st, "alice", "secret-password")

	_, err := svc.Login("alice", "WRONG", "", "1.2.3.4", "")
	if err != auth.ErrBadCredentials {
		t.Fatalf("expected ErrBadCredentials, got %v", err)
	}
}

func TestLogin_UnknownUser_HashesAnyway(t *testing.T) {
	// Timing-attack mitigation: even with no user, login must spend Argon2 time
	// equivalent to a real verification. The interface promise is just that we
	// return the same error code as a wrong password.
	svc, _, _ := setupService(t)
	_, err := svc.Login("nobody", "anything", "", "1.2.3.4", "")
	if err != auth.ErrBadCredentials {
		t.Fatalf("expected ErrBadCredentials, got %v", err)
	}
}

func TestLogin_LockoutAfterMaxAttempts(t *testing.T) {
	svc, st, cfg := setupService(t)
	createUser(t, st, "alice", "secret-password")

	for i := 0; i < cfg.Security.MaxLoginAttempts; i++ {
		if _, err := svc.Login("alice", "WRONG", "", "1.1.1.1", ""); err != auth.ErrBadCredentials {
			t.Fatalf("attempt %d: expected ErrBadCredentials, got %v", i, err)
		}
	}
	// The next attempt — even with correct password — must hit lockout.
	if _, err := svc.Login("alice", "secret-password", "", "1.1.1.1", ""); err != auth.ErrLockedOut {
		t.Fatalf("expected ErrLockedOut, got %v", err)
	}
}

func TestLogin_LockoutIsPerIP(t *testing.T) {
	svc, st, cfg := setupService(t)
	createUser(t, st, "alice", "secret-password")

	// Burn the lockout for one IP.
	for i := 0; i < cfg.Security.MaxLoginAttempts; i++ {
		_, _ = svc.Login("alice", "WRONG", "", "10.0.0.1", "")
	}
	// A different IP must still be able to log in.
	if _, err := svc.Login("alice", "secret-password", "", "10.0.0.2", ""); err != nil {
		t.Fatalf("second IP locked out: %v", err)
	}
}

func TestLogin_TOTPRequiredWhenEnrolled(t *testing.T) {
	svc, st, _ := setupService(t)
	u := createUser(t, st, "alice", "secret-password")
	// Manually enable 2FA for the user.
	key, _ := auth.GenerateTOTPSecret("test", u.Username)
	if err := st.UpdateUserTOTP(u.ID, key.Secret()); err != nil {
		t.Fatal(err)
	}
	// No code → ErrTOTPRequired.
	if _, err := svc.Login("alice", "secret-password", "", "1.1.1.1", ""); err != auth.ErrTOTPRequired {
		t.Fatalf("expected ErrTOTPRequired without code, got %v", err)
	}
	// Wrong code → ErrTOTPInvalid.
	if _, err := svc.Login("alice", "secret-password", "000000", "1.1.1.1", ""); err != auth.ErrTOTPInvalid {
		t.Fatalf("expected ErrTOTPInvalid with bad code, got %v", err)
	}
}

func TestLookupSession_RejectsExpired(t *testing.T) {
	svc, st, _ := setupService(t)
	createUser(t, st, "alice", "secret-password")
	res, err := svc.Login("alice", "secret-password", "", "1.1.1.1", "")
	if err != nil {
		t.Fatal(err)
	}
	// Manually expire the session via direct DB write.
	if _, err := st.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Hour).Unix(), res.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.LookupSession(res.Session.ID); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound for expired session, got %v", err)
	}
}
