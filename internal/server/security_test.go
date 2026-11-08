package server

import (
	"errors"
	"io"
	"net/http"
	"os/user"
	"testing"

	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/store"
)

func TestSetup_RequiresOneTimeToken(t *testing.T) {
	srv, h := newTestServer(t)
	if srv.setupToken == "" {
		t.Fatal("fresh server must generate a setup token")
	}
	body := func(token string) map[string]string {
		return map[string]string{"username": "admin", "password": "correct-horse-battery", "setup_token": token}
	}

	for _, tok := range []string{"", "wrong-token"} {
		res := doJSON(t, h, http.MethodPost, "/api/auth/setup", body(tok), nil, nil)
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("setup with token %q should 403, got %d", tok, res.StatusCode)
		}
	}
	if n, _ := srv.Store.CountUsers(); n != 0 {
		t.Fatalf("rejected setup must not create users, got %d", n)
	}

	res := doJSON(t, h, http.MethodPost, "/api/auth/setup", body(srv.setupToken), nil, nil)
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("setup with valid token: %d %s", res.StatusCode, b)
	}
	if srv.setupToken != "" {
		t.Fatal("setup token must be cleared after use")
	}
}

func TestSetup_NoTokenWhenUsersExist(t *testing.T) {
	srv, _ := newTestServer(t)
	if _, err := srv.Store.CreateUser("admin", "x", "", "admin", true); err != nil {
		t.Fatal(err)
	}
	srv2 := New(srv.Cfg, srv.Store, srv.Log, nil)
	if srv2.setupToken != "" {
		t.Fatal("configured server must not expose a setup token")
	}
}

func TestSession_CookieIsNotTheStoredID(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, _ := loginAs(t, srv, h, "admin1", "admin")
	if _, err := srv.Store.GetSession(cookie.Value); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("plaintext cookie must not be a DB key, got %v", err)
	}
	if _, err := srv.Store.GetSession(auth.HashSessionID(cookie.Value)); err != nil {
		t.Fatalf("session should be stored under the token hash: %v", err)
	}
	res := doJSON(t, h, http.MethodGet, "/api/auth/me", nil, nil, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		t.Fatalf("cookie should still authenticate, got %d", res.StatusCode)
	}
}

func TestRBAC_FileManagerIsAdminOnly(t *testing.T) {
	srv, h := newTestServer(t)
	for _, role := range []string{"viewer", "operator"} {
		cookie, _ := loginAs(t, srv, h, role+"1", role)
		for _, path := range []string{
			"/api/files/list?path=/etc",
			"/api/files/read?path=/etc/shadow",
			"/api/files/download?path=/etc/shadow",
		} {
			res := doJSON(t, h, http.MethodGet, path, nil, nil, []*http.Cookie{cookie})
			if res.StatusCode != http.StatusForbidden {
				t.Errorf("%s GET %s should 403, got %d", role, path, res.StatusCode)
			}
		}
	}
	cookie, _ := loginAs(t, srv, h, "admin1", "admin")
	res := doJSON(t, h, http.MethodGet, "/api/files/list?path=/", nil, nil, []*http.Cookie{cookie})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("admin list / should 200, got %d", res.StatusCode)
	}
}

func TestRBAC_SecretBearingReadsNotForViewer(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, _ := loginAs(t, srv, h, "viewer1", "viewer")
	for _, path := range []string{
		"/api/docker/containers/abc123",
		"/api/docker/containers/abc123/logs/stream",
		"/api/services/ssh.service/journal/stream",
		"/api/cron/root",
		"/api/stacks/1",
		"/api/backups/1/download",
	} {
		res := doJSON(t, h, http.MethodGet, path, nil, nil, []*http.Cookie{cookie})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("viewer GET %s should 403, got %d", path, res.StatusCode)
		}
	}
}

func TestRBAC_HostEscapeIsAdminOnly(t *testing.T) {
	srv, h := newTestServer(t)
	opCookie, opCSRF := loginAs(t, srv, h, "op1", "operator")
	adCookie, adCSRF := loginAs(t, srv, h, "admin1", "admin")

	escapes := []map[string]any{
		{"name": "p", "image": "alpine", "privileged": true},
		{"name": "n", "image": "alpine", "network_mode": "host"},
		{"name": "b", "image": "alpine", "mounts": []map[string]any{
			{"type": "bind", "source": "/", "target": "/host"}}},
	}
	for _, spec := range escapes {
		res := doJSON(t, h, http.MethodPost, "/api/docker/containers", spec,
			map[string]string{auth.CSRFHeaderName: opCSRF}, []*http.Cookie{opCookie})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("operator create %v should 403, got %d", spec["name"], res.StatusCode)
		}
		// Admins pass the check (Docker itself is unreachable in tests).
		res = doJSON(t, h, http.MethodPost, "/api/docker/containers", spec,
			map[string]string{auth.CSRFHeaderName: adCSRF}, []*http.Cookie{adCookie})
		if res.StatusCode == http.StatusForbidden {
			t.Errorf("admin create %v should not 403", spec["name"])
		}
	}

	plain := map[string]any{"name": "ok", "image": "alpine", "mounts": []map[string]any{
		{"type": "volume", "source": "data", "target": "/data"}}}
	res := doJSON(t, h, http.MethodPost, "/api/docker/containers", plain,
		map[string]string{auth.CSRFHeaderName: opCSRF}, []*http.Cookie{opCookie})
	if res.StatusCode == http.StatusForbidden {
		t.Errorf("operator create with named volume should not 403")
	}

	stack := map[string]string{"name": "evil", "compose": "services:\n  x:\n    image: alpine\n    volumes:\n      - /:/host\n"}
	res = doJSON(t, h, http.MethodPost, "/api/stacks", stack,
		map[string]string{auth.CSRFHeaderName: opCSRF}, []*http.Cookie{opCookie})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("operator stack with host bind should 403, got %d", res.StatusCode)
	}
}

func TestCron_OperatorCannotTouchForeignCrontab(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "op1", "operator")
	hdr := map[string]string{auth.CSRFHeaderName: csrf}
	for _, target := range []string{"root", "someoneelse"} {
		res := doJSON(t, h, http.MethodGet, "/api/cron/"+target, nil, nil, []*http.Cookie{cookie})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("operator list %s crontab should 403, got %d", target, res.StatusCode)
		}
		res = doJSON(t, h, http.MethodPost, "/api/cron/"+target,
			map[string]string{"schedule": "* * * * *", "command": "id"}, hdr, []*http.Cookie{cookie})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("operator add to %s crontab should 403, got %d", target, res.StatusCode)
		}
	}
}

func TestTerminalAccount(t *testing.T) {
	lookup := func(name string) (*user.User, error) {
		switch name {
		case "alice":
			return &user.User{Username: "alice", Uid: "1000"}, nil
		case "root":
			return &user.User{Username: "root", Uid: "0"}, nil
		}
		return nil, user.UnknownUserError(name)
	}
	cases := []struct {
		name, role, want string
		wantErr          bool
	}{
		{"alice", "operator", "alice", false},
		{"ghost", "operator", "", true}, // no Linux account: refuse, never root
		{"root", "operator", "", true},  // same-named root account: admin only
		{"ghost", "admin", "", false},   // admin falls back to the process user
		{"root", "admin", "root", false},
	}
	for _, c := range cases {
		got, err := terminalAccount(&store.User{Username: c.name, Role: c.role}, lookup)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%s/%s: got (%q, %v), want (%q, err=%v)", c.name, c.role, got, err, c.want, c.wantErr)
		}
	}
}

func TestRBAC_PackagePlanIsAdminOnly(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "op1", "operator")
	res := doJSON(t, h, http.MethodPost, "/api/packages/plan",
		map[string]any{"op": "install", "names": []string{"ufw"}},
		map[string]string{auth.CSRFHeaderName: csrf}, []*http.Cookie{cookie})
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("operator plan should 403, got %d", res.StatusCode)
	}
}
