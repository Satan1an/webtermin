package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Satan1an/webtermin/internal/auth"
)

// newToken issues an API token with the given role for a fresh admin user.
func newToken(t *testing.T, srv *Server, h http.Handler, role string) string {
	t.Helper()
	cookie, csrf := loginAs(t, srv, h, "owner-"+role, "admin")
	res := doJSON(t, h, http.MethodPost, "/api/panel/tokens",
		map[string]any{"name": "agent-" + role, "role": role},
		map[string]string{auth.CSRFHeaderName: csrf}, []*http.Cookie{cookie})
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create token: %d %s", res.StatusCode, b)
	}
	var out struct {
		Plaintext string `json:"plaintext"`
	}
	decode(t, res, &out)
	return out.Plaintext
}

// mcpCall sends one JSON-RPC request to /api/mcp and returns the decoded
// envelope. The endpoint is stateless, so no initialize handshake is needed.
func mcpCall(t *testing.T, h http.Handler, token string, cookies []*http.Cookie, method string, params any) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, c := range cookies {
		if c.Name == "csrf" { // pseudo-cookie: carries the CSRF header for session calls
			req.Header.Set(auth.CSRFHeaderName, c.Value)
			continue
		}
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env
}

func mcpToolNames(t *testing.T, env map[string]any) []string {
	t.Helper()
	res, _ := env["result"].(map[string]any)
	tools, _ := res["tools"].([]any)
	var names []string
	for _, tl := range tools {
		names = append(names, tl.(map[string]any)["name"].(string))
	}
	if len(names) == 0 {
		t.Fatalf("no tools in response: %v", env)
	}
	return names
}

func TestMCP_RejectsBrowserSession(t *testing.T) {
	srv, h := newTestServer(t)
	cookie, csrf := loginAs(t, srv, h, "admin1", "admin")
	// A valid session with a valid CSRF token still must not reach MCP.
	code, _ := mcpCall(t, h, "", []*http.Cookie{cookie, {Name: "csrf", Value: csrf}}, "tools/list", map[string]any{})
	if code != http.StatusUnauthorized {
		t.Fatalf("MCP over a session cookie must be refused, got %d", code)
	}
	if code, _ := mcpCall(t, h, "", nil, "tools/list", map[string]any{}); code != http.StatusUnauthorized {
		t.Fatalf("anonymous MCP must be refused, got %d", code)
	}
}

func TestMCP_ToolsFollowTokenRole(t *testing.T) {
	srv, h := newTestServer(t)
	cases := []struct {
		role      string
		must, not []string
	}{
		{"viewer", []string{"host_status", "list_services", "list_containers", "list_updates"},
			[]string{"service_logs", "container_logs", "package_plan", "recent_audit", "network_interfaces"}},
		{"operator", []string{"host_status", "service_logs", "container_logs"},
			[]string{"package_plan", "recent_audit", "network_interfaces"}},
		{"admin", []string{"host_status", "service_logs", "package_plan", "recent_audit", "network_interfaces"}, nil},
	}
	for _, c := range cases {
		code, env := mcpCall(t, h, newToken(t, srv, h, c.role), nil, "tools/list", map[string]any{})
		if code != 200 {
			t.Fatalf("%s tools/list: %d %v", c.role, code, env)
		}
		names := mcpToolNames(t, env)
		for _, n := range c.must {
			if !slices.Contains(names, n) {
				t.Errorf("%s token should see %s; got %v", c.role, n, names)
			}
		}
		for _, n := range c.not {
			if slices.Contains(names, n) {
				t.Errorf("%s token must not see %s", c.role, n)
			}
		}
	}
}

func TestMCP_ViewerCannotCallHiddenTool(t *testing.T) {
	srv, h := newTestServer(t)
	_, env := mcpCall(t, h, newToken(t, srv, h, "viewer"), nil, "tools/call",
		map[string]any{"name": "package_plan", "arguments": map[string]any{"op": "install", "names": []string{"htop"}}})
	if env["error"] == nil {
		if res, _ := env["result"].(map[string]any); res["isError"] != true {
			t.Fatalf("viewer calling package_plan must fail, got %v", env)
		}
	}
}

func TestMCP_CallsAreAudited(t *testing.T) {
	srv, h := newTestServer(t)
	code, env := mcpCall(t, h, newToken(t, srv, h, "admin"), nil, "tools/call",
		map[string]any{"name": "recent_audit", "arguments": map[string]any{"limit": 5}})
	if code != 200 || env["error"] != nil {
		t.Fatalf("recent_audit: %d %v", code, env)
	}
	entries, err := srv.Store.ListAudit(1)
	if err != nil || len(entries) == 0 {
		t.Fatal(err)
	}
	e := entries[0]
	if e.Action != "mcp.recent_audit" || e.Outcome != "ok" || !strings.Contains(e.Detail, "token=agent-admin") {
		t.Errorf("last audit entry = %+v", e)
	}
}
