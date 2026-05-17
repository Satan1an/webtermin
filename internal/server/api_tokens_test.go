package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Satan1an/webtermin/internal/auth"
)

// reqWithToken builds an httptest request that authenticates with a bearer
// token instead of a session cookie. CSRF header is intentionally omitted to
// confirm the middleware skips it for token clients.
func reqWithToken(t *testing.T, h http.Handler, method, path, plaintext string, body any) *http.Response {
	t.Helper()
	headers := map[string]string{"Authorization": "Bearer " + plaintext}
	return doJSON(t, h, method, path, body, headers, nil)
}

func TestAPIToken_CreateAndUse(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "admin1", "admin")

	// Admin creates an operator-role token for themselves.
	res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
		map[string]any{"name": "ci-bot", "role": "operator"},
		map[string]string{auth.CSRFHeaderName: csrf}, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("create token: %d %s", res.StatusCode, body)
	}
	var created struct {
		Token struct {
			ID   int64  `json:"id"`
			Role string `json:"role"`
		} `json:"token"`
		Plaintext string `json:"plaintext"`
	}
	decode(t, res, &created)
	if !strings.HasPrefix(created.Plaintext, "wt_") {
		t.Fatalf("plaintext prefix: %q", created.Plaintext)
	}
	if created.Token.Role != "operator" {
		t.Fatalf("issued role: %q", created.Token.Role)
	}

	// Use the token to hit an authenticated read endpoint — no cookie, no CSRF.
	res = reqWithToken(t, h, http.MethodGet, "/api/auth/me", created.Plaintext, nil)
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("token GET /me: %d %s", res.StatusCode, body)
	}

	// Token can do an operator action without CSRF.
	res = reqWithToken(t, h, http.MethodPost, "/api/files/mkdir",
		created.Plaintext, map[string]string{"path": "/tmp/wt-token-test-" + auth.RandomToken(4)})
	// SafePath will accept the absolute path; we only assert NOT 401/403/400-from-csrf.
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("token operator mkdir got auth-error: %d %s", res.StatusCode, body)
	}
}

func TestAPIToken_RoleCap(t *testing.T) {
	srv, h := newTestServer(t)
	// Login as operator — must NOT be able to mint an admin-role token.
	cookie, csrf := loginAs(t, srv, h, "op1", "operator")
	hdr := map[string]string{auth.CSRFHeaderName: csrf}

	res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
		map[string]any{"name": "escalate", "role": "admin"}, hdr, []*http.Cookie{cookie})
	if res.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("operator->admin token should 403, got %d %s", res.StatusCode, body)
	}

	// But operator can issue an operator-or-viewer-role token.
	for _, role := range []string{"viewer", "operator"} {
		res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
			map[string]any{"name": "ok-" + role, "role": role}, hdr, []*http.Cookie{cookie})
		if res.StatusCode != 200 {
			body, _ := io.ReadAll(res.Body)
			t.Errorf("operator->%s token should succeed, got %d %s", role, res.StatusCode, body)
		}
	}
}

func TestAPIToken_ScopeIsHonored(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "admin1", "admin")

	// Admin issues a VIEWER-role token bound to their own account.
	res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
		map[string]any{"name": "viewer-bot", "role": "viewer"},
		map[string]string{auth.CSRFHeaderName: csrf}, []*http.Cookie{cookie})
	var created struct {
		Plaintext string `json:"plaintext"`
	}
	decode(t, res, &created)

	// Using the viewer token, a mutating call must 403 — token role clamps
	// the owner's effective role for the request.
	res = reqWithToken(t, h, http.MethodPost,
		"/api/services/nginx.service/action",
		created.Plaintext, map[string]string{"action": "restart"})
	if res.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("viewer token mutating action should 403, got %d %s", res.StatusCode, body)
	}
}

func TestAPIToken_Revoke(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "admin1", "admin")
	hdr := map[string]string{auth.CSRFHeaderName: csrf}

	res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
		map[string]any{"name": "soon-dead", "role": "viewer"}, hdr, []*http.Cookie{cookie})
	var created struct {
		Token struct {
			ID int64 `json:"id"`
		} `json:"token"`
		Plaintext string `json:"plaintext"`
	}
	decode(t, res, &created)

	// It works first…
	res = reqWithToken(t, h, http.MethodGet, "/api/auth/me", created.Plaintext, nil)
	if res.StatusCode != 200 {
		t.Fatalf("token should work before revoke: %d", res.StatusCode)
	}

	// Admin revokes it.
	res = doJSON(t, h, http.MethodDelete,
		"/api/panel/tokens/"+intStr(int(created.Token.ID)),
		nil, hdr, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("revoke: %d %s", res.StatusCode, body)
	}

	// …and stops working.
	res = reqWithToken(t, h, http.MethodGet, "/api/auth/me", created.Plaintext, nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked token should 401, got %d", res.StatusCode)
	}
}

func TestAPIToken_BadTokenRejected(t *testing.T) {
	_, h := newTestServer(t)
	// Token-shaped but unknown.
	res := reqWithToken(t, h, http.MethodGet, "/api/auth/me", "wt_"+strings.Repeat("a", 43), nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown token should 401, got %d", res.StatusCode)
	}
}

func TestAPIToken_NonOwnerCannotRevoke(t *testing.T) {
	srv, h := newTestServer(t)
	// User A issues a token.
	cookieA, csrfA := loginAs(t, srv, h, "alice", "operator")
	res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
		map[string]any{"name": "alice-tok", "role": "operator"},
		map[string]string{auth.CSRFHeaderName: csrfA}, []*http.Cookie{cookieA})
	var created struct {
		Token struct {
			ID int64 `json:"id"`
		} `json:"token"`
	}
	decode(t, res, &created)

	// User B (also operator, NOT admin) cannot revoke it.
	cookieB, csrfB := loginAs(t, srv, h, "bob", "operator")
	res = doJSON(t, h, http.MethodDelete,
		"/api/panel/tokens/"+intStr(int(created.Token.ID)), nil,
		map[string]string{auth.CSRFHeaderName: csrfB}, []*http.Cookie{cookieB})
	if res.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("non-owner non-admin revoke should 403, got %d %s", res.StatusCode, body)
	}
}

// Silence the json import warning if it ever ends up unused after edits.
var _ = json.Marshal
