package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/store"
)

// loginAs creates a user with the given role and returns the session cookie
// and CSRF token ready for use in subsequent requests.
func loginAs(t *testing.T, srv *Server, h http.Handler, username, role string) (*http.Cookie, string) {
	t.Helper()
	pw := "test-password-1234"
	hash, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Store.CreateUser(username, hash, "", role, role == "admin"); err != nil {
		t.Fatal(err)
	}
	res := doJSON(t, h, http.MethodPost, "/api/auth/login",
		map[string]string{"username": username, "password": pw}, nil, nil)
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("login as %s/%s failed: %d %s", username, role, res.StatusCode, body)
	}
	var info struct {
		CSRFToken string `json:"csrf_token"`
	}
	decode(t, res, &info)
	cookie := cookieFrom(res, auth.SessionCookieName)
	if cookie == nil {
		t.Fatal("no session cookie")
	}
	return cookie, info.CSRFToken
}

func TestRBAC_ViewerCannotMutate(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "viewer1", "viewer")

	// Reads should pass.
	res := doJSON(t, h, http.MethodGet, "/api/auth/me", nil, nil, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		t.Fatalf("viewer /me: got %d", res.StatusCode)
	}

	// Writes should 403.
	mutating := []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/services/nginx.service/action", map[string]string{"action": "restart"}},
		{"POST", "/api/files/write", map[string]string{"path": "/tmp/x", "content": "y"}},
		{"POST", "/api/files/mkdir", map[string]string{"path": "/tmp/x"}},
		{"DELETE", "/api/files/delete", map[string]string{"path": "/tmp/x"}},
	}
	for _, m := range mutating {
		res := doJSON(t, h, m.method, m.path, m.body,
			map[string]string{auth.CSRFHeaderName: csrf}, []*http.Cookie{cookie})
		if res.StatusCode != http.StatusForbidden {
			body, _ := io.ReadAll(res.Body)
			t.Errorf("viewer %s %s should 403, got %d (%s)", m.method, m.path, res.StatusCode, body)
		}
	}
}

func TestRBAC_OperatorCanMutateButNotManageUsers(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "op1", "operator")

	// Panel-user management endpoints are admin-only.
	adminOnly := []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/panel/users", nil},
		{"POST", "/api/panel/users",
			map[string]string{"username": "x", "password": "yyyyyyyyyy", "role": "viewer"}},
		{"GET", "/api/auth/audit", nil},
		{"POST", "/api/linux/users", map[string]string{"name": "test"}},
		{"DELETE", "/api/linux/users/test", nil},
	}
	for _, m := range adminOnly {
		var headers map[string]string
		if m.method != http.MethodGet {
			headers = map[string]string{auth.CSRFHeaderName: csrf}
		}
		res := doJSON(t, h, m.method, m.path, m.body, headers, []*http.Cookie{cookie})
		if res.StatusCode != http.StatusForbidden {
			body, _ := io.ReadAll(res.Body)
			t.Errorf("operator %s %s should 403, got %d (%s)", m.method, m.path, res.StatusCode, body)
		}
	}
}

func TestRBAC_AdminCanManagePanelUsers(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "admin1", "admin")
	hdr := map[string]string{auth.CSRFHeaderName: csrf}

	// Create a viewer.
	res := doJSON(t, h, http.MethodPost, "/api/panel/users",
		map[string]string{"username": "newviewer", "password": "viewer-pwd-1234", "role": "viewer"},
		hdr, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("admin create user: %d %s", res.StatusCode, body)
	}
	var created struct {
		ID   int64  `json:"id"`
		Role string `json:"role"`
	}
	decode(t, res, &created)
	if created.Role != "viewer" {
		t.Fatalf("created role: got %q, want viewer", created.Role)
	}

	// Promote them to operator.
	res = doJSON(t, h, http.MethodPost,
		"/api/panel/users/"+itoa(int(created.ID))+"/role",
		map[string]string{"role": "operator"}, hdr, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("set role: %d %s", res.StatusCode, body)
	}
	u, _ := srv.Store.GetUser(created.ID)
	if u.Role != "operator" {
		t.Fatalf("role not persisted: %q", u.Role)
	}
}

func TestRBAC_CannotRemoveLastAdmin(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "onlyadmin", "admin")
	hdr := map[string]string{auth.CSRFHeaderName: csrf}

	u, _ := srv.Store.GetUserByName("onlyadmin")

	// Can't delete self at all.
	res := doJSON(t, h, http.MethodDelete,
		"/api/panel/users/"+itoa(int(u.ID)),
		nil, hdr, []*http.Cookie{cookie})
	if res.StatusCode != 400 {
		t.Fatalf("delete self should 400, got %d", res.StatusCode)
	}

	// Add a second user (viewer), then can't downgrade the only admin.
	if _, err := srv.Store.CreateUser("v", "x", "", "viewer", false); err != nil {
		t.Fatal(err)
	}

	res = doJSON(t, h, http.MethodPost,
		"/api/panel/users/"+itoa(int(u.ID))+"/role",
		map[string]string{"role": "viewer"}, hdr, []*http.Cookie{cookie})
	if res.StatusCode != 400 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("downgrade last admin should 400, got %d (%s)", res.StatusCode, body)
	}
	// And the role hasn't actually changed.
	u, _ = srv.Store.GetUserByName("onlyadmin")
	if u.Role != "admin" {
		t.Fatalf("last-admin role mutated to %q", u.Role)
	}
}

// itoa avoids strconv import in test files.
func itoa(n int) string {
	return strings.TrimLeft(intStr(n), "+")
}

func intStr(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// silence unused-import warning if store is only used via type assertion
// in some sub-tests:
var _ = store.ErrNotFound
