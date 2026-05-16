package server

import (
	"io/fs"
	"net/http"
	"strings"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints — no session required.
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/setup", s.handleFirstRunSetup)

	// Everything below requires a valid session + CSRF token.
	authed := http.NewServeMux()
	authed.HandleFunc("POST /api/auth/logout", s.handleLogout)
	authed.HandleFunc("GET /api/auth/me", s.handleMe)
	authed.HandleFunc("GET /api/auth/audit", s.handleAuditList)

	authed.HandleFunc("POST /api/auth/2fa/enroll", s.handle2FAEnroll)
	authed.HandleFunc("POST /api/auth/2fa/verify", s.handle2FAVerify)
	authed.HandleFunc("POST /api/auth/2fa/disable", s.handle2FADisable)

	authed.HandleFunc("GET /api/system/info", s.handleSystemInfo)
	authed.HandleFunc("GET /api/system/metrics", s.handleSystemMetrics)
	authed.HandleFunc("GET /api/system/metrics/stream", s.handleSystemMetricsStream)

	authed.HandleFunc("GET /api/services", s.handleServicesList)
	authed.HandleFunc("POST /api/services/{name}/action", s.handleServiceAction)
	authed.HandleFunc("GET /api/services/{name}/journal/stream", s.handleServiceJournalStream)

	authed.HandleFunc("GET /api/linux/users", s.handleLinuxUsersList)
	authed.HandleFunc("POST /api/linux/users", s.handleLinuxUserCreate)
	authed.HandleFunc("DELETE /api/linux/users/{name}", s.handleLinuxUserDelete)
	authed.HandleFunc("POST /api/linux/users/{name}/password", s.handleLinuxUserPassword)
	authed.HandleFunc("GET /api/linux/users/{name}/keys", s.handleLinuxUserKeysList)
	authed.HandleFunc("POST /api/linux/users/{name}/keys", s.handleLinuxUserKeyAdd)
	authed.HandleFunc("DELETE /api/linux/users/{name}/keys/{fp}", s.handleLinuxUserKeyDelete)

	authed.HandleFunc("GET /api/files/list", s.handleFilesList)
	authed.HandleFunc("GET /api/files/read", s.handleFileRead)
	authed.HandleFunc("POST /api/files/write", s.handleFileWrite)
	authed.HandleFunc("POST /api/files/upload", s.handleFileUpload)
	authed.HandleFunc("GET /api/files/download", s.handleFileDownload)
	authed.HandleFunc("POST /api/files/mkdir", s.handleFileMkdir)
	authed.HandleFunc("DELETE /api/files/delete", s.handleFileDelete)
	authed.HandleFunc("POST /api/files/chmod", s.handleFileChmod)

	authed.HandleFunc("GET /api/terminal/ws", s.handleTerminalWS)

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
