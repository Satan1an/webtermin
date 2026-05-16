package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/config"
	"github.com/Satan1an/webtermin/internal/store/storetest"
)

// helper builds a real Server wired against an in-memory store, plus the http
// handler tree it would serve in production.
func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	cfg := config.Default()
	cfg.Security.MaxLoginAttempts = 3
	cfg.Security.LockoutMin = 1
	cfg.Security.SessionTTLMin = 60
	st := storetest.New(t)
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(cfg, st, lg, nil)
	return srv, srv.securityHeaders(srv.routes())
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func decode(t *testing.T, res *http.Response, dst any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func cookieFrom(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestEndToEnd_SetupLoginMutateLogout(t *testing.T) {
	_, h := newTestServer(t)

	// 1. Status — needs setup.
	res := doJSON(t, h, http.MethodGet, "/api/auth/status", nil, nil, nil)
	var status struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	decode(t, res, &status)
	if !status.NeedsSetup {
		t.Fatal("fresh server should report needs_setup=true")
	}

	// 2. Setup admin.
	res = doJSON(t, h, http.MethodPost, "/api/auth/setup",
		map[string]string{"username": "admin", "password": "correct-horse-battery"},
		nil, nil)
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("setup failed: %d %s", res.StatusCode, body)
	}

	// 3. Setup again — must be rejected.
	res = doJSON(t, h, http.MethodPost, "/api/auth/setup",
		map[string]string{"username": "admin2", "password": "x"}, nil, nil)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("second setup should 409, got %d", res.StatusCode)
	}

	// 4. Login with wrong password.
	res = doJSON(t, h, http.MethodPost, "/api/auth/login",
		map[string]string{"username": "admin", "password": "WRONG"}, nil, nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password should 401, got %d", res.StatusCode)
	}

	// 5. Login correctly.
	res = doJSON(t, h, http.MethodPost, "/api/auth/login",
		map[string]string{"username": "admin", "password": "correct-horse-battery"}, nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d", res.StatusCode)
	}
	var session struct {
		User      string `json:"user"`
		CSRFToken string `json:"csrf_token"`
	}
	sessionCookie := cookieFrom(res, auth.SessionCookieName)
	if sessionCookie == nil {
		t.Fatal("no session cookie issued")
	}
	if !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie has weak attributes: %+v", sessionCookie)
	}
	decode(t, res, &session)
	if session.User != "admin" || session.CSRFToken == "" {
		t.Fatalf("bad session payload: %+v", session)
	}

	cookies := []*http.Cookie{sessionCookie}

	// 6. /api/auth/me with session — OK.
	res = doJSON(t, h, http.MethodGet, "/api/auth/me", nil, nil, cookies)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("/me should succeed, got %d", res.StatusCode)
	}

	// 7. Mutating request WITHOUT CSRF — must 403.
	res = doJSON(t, h, http.MethodPost, "/api/auth/logout", nil, nil, cookies)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("logout without CSRF should 403, got %d", res.StatusCode)
	}

	// 8. Mutating request WITH wrong CSRF — must 403.
	res = doJSON(t, h, http.MethodPost, "/api/auth/logout", nil,
		map[string]string{auth.CSRFHeaderName: "bogus"}, cookies)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("logout with bad CSRF should 403, got %d", res.StatusCode)
	}

	// 9. Mutating request WITH correct CSRF — OK.
	res = doJSON(t, h, http.MethodPost, "/api/auth/logout", nil,
		map[string]string{auth.CSRFHeaderName: session.CSRFToken}, cookies)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("logout with good CSRF should succeed, got %d", res.StatusCode)
	}

	// 10. Same session must now be rejected.
	res = doJSON(t, h, http.MethodGet, "/api/auth/me", nil, nil, cookies)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/me after logout should 401, got %d", res.StatusCode)
	}
}

func TestSecurityHeaders_Present(t *testing.T) {
	_, h := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %s: got %q, want %q", k, got, v)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, must := range []string{"default-src 'self'", "frame-ancestors 'none'", "base-uri 'self'"} {
		if !strings.Contains(csp, must) {
			t.Errorf("CSP missing %q. Full: %s", must, csp)
		}
	}
}
