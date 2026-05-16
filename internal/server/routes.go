package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/Satan1an/webtermin/internal/auth"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints — no session required.
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/setup", s.handleFirstRunSetup)

	// Everything below requires a valid session + CSRF token. Per-endpoint
	// role policy: viewer is the floor, operator can perform write actions
	// (start/stop services, manage files, open terminals), admin can manage
	// panel users + Linux system users and read the audit log.
	authed := http.NewServeMux()

	// Session basics — any authed user.
	authed.HandleFunc("POST /api/auth/logout", s.handleLogout)
	authed.HandleFunc("GET /api/auth/me", s.handleMe)

	// 2FA enrollment manages own account — any authed user.
	authed.HandleFunc("POST /api/auth/2fa/enroll", s.handle2FAEnroll)
	authed.HandleFunc("POST /api/auth/2fa/verify", s.handle2FAVerify)
	authed.HandleFunc("POST /api/auth/2fa/disable", s.handle2FADisable)

	// Audit log is sensitive — admin-only.
	authed.HandleFunc("GET /api/auth/audit", s.protected(auth.RoleAdmin, s.handleAuditList))

	// Panel-user management (this is the RBAC self-management surface).
	authed.HandleFunc("GET /api/panel/users", s.protected(auth.RoleAdmin, s.handlePanelUsersList))
	authed.HandleFunc("POST /api/panel/users", s.protected(auth.RoleAdmin, s.handlePanelUserCreate))
	authed.HandleFunc("DELETE /api/panel/users/{id}", s.protected(auth.RoleAdmin, s.handlePanelUserDelete))
	authed.HandleFunc("POST /api/panel/users/{id}/role", s.protected(auth.RoleAdmin, s.handlePanelUserSetRole))
	authed.HandleFunc("POST /api/panel/users/{id}/password", s.protected(auth.RoleAdmin, s.handlePanelUserSetPassword))

	// Read-only system metrics — viewer is enough.
	authed.HandleFunc("GET /api/system/info", s.handleSystemInfo)
	authed.HandleFunc("GET /api/system/metrics", s.handleSystemMetrics)
	authed.HandleFunc("GET /api/system/metrics/stream", s.handleSystemMetricsStream)

	// systemd — list/journal anyone authed; start/stop is an Operator action.
	authed.HandleFunc("GET /api/services", s.handleServicesList)
	authed.HandleFunc("GET /api/services/{name}/journal/stream", s.handleServiceJournalStream)
	authed.HandleFunc("POST /api/services/{name}/action", s.protected(auth.RoleOperator, s.handleServiceAction))

	// Linux system users — listing is operator+; mutating is admin-only because
	// adding/removing Linux accounts has system-wide blast radius.
	authed.HandleFunc("GET /api/linux/users", s.protected(auth.RoleOperator, s.handleLinuxUsersList))
	authed.HandleFunc("POST /api/linux/users", s.protected(auth.RoleAdmin, s.handleLinuxUserCreate))
	authed.HandleFunc("DELETE /api/linux/users/{name}", s.protected(auth.RoleAdmin, s.handleLinuxUserDelete))
	authed.HandleFunc("POST /api/linux/users/{name}/password", s.protected(auth.RoleAdmin, s.handleLinuxUserPassword))
	authed.HandleFunc("GET /api/linux/users/{name}/keys", s.protected(auth.RoleOperator, s.handleLinuxUserKeysList))
	authed.HandleFunc("POST /api/linux/users/{name}/keys", s.protected(auth.RoleAdmin, s.handleLinuxUserKeyAdd))
	authed.HandleFunc("DELETE /api/linux/users/{name}/keys/{fp}", s.protected(auth.RoleAdmin, s.handleLinuxUserKeyDelete))

	// Files — reads to anyone authed, mutations operator+.
	authed.HandleFunc("GET /api/files/list", s.handleFilesList)
	authed.HandleFunc("GET /api/files/read", s.handleFileRead)
	authed.HandleFunc("GET /api/files/download", s.handleFileDownload)
	authed.HandleFunc("POST /api/files/write", s.protected(auth.RoleOperator, s.handleFileWrite))
	authed.HandleFunc("POST /api/files/upload", s.protected(auth.RoleOperator, s.handleFileUpload))
	authed.HandleFunc("POST /api/files/mkdir", s.protected(auth.RoleOperator, s.handleFileMkdir))
	authed.HandleFunc("DELETE /api/files/delete", s.protected(auth.RoleOperator, s.handleFileDelete))
	authed.HandleFunc("POST /api/files/chmod", s.protected(auth.RoleOperator, s.handleFileChmod))

	// Terminal — operator+. A PTY is effectively a shell-exec on the host.
	authed.HandleFunc("GET /api/terminal/ws", s.protected(auth.RoleOperator, s.handleTerminalWS))

	mux.Handle("/api/auth/logout", s.requireSession(s.requireCSRF(authed)))
	mux.Handle("/api/", s.requireSession(s.requireCSRF(authed)))

	if s.WebFS != nil {
		mux.Handle("/", s.spaHandler())
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not embedded — run `make web` then rebuild", http.StatusNotImplemented)
		})
	}

	return mux
}

// spaHandler serves static assets from the embedded FS and falls back to
// index.html for any non-asset path (client-side router).
func (s *Server) spaHandler() http.Handler {
	staticServer := http.FileServer(http.FS(s.WebFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never serve SPA for /api/* — those should already have matched.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, err := fs.Stat(s.WebFS, clean); err == nil {
			staticServer.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		staticServer.ServeHTTP(w, r2)
	})
}
