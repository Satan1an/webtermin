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

	// OIDC SSO is public by definition — start kicks off the redirect dance,
	// callback verifies and issues a session cookie.
	mux.HandleFunc("GET /api/auth/oidc/status", s.handleOIDCStatus)
	mux.HandleFunc("GET /api/auth/oidc/start", s.handleOIDCStart)
	mux.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)

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

	// API tokens — any authed user manages their own; admins see/revoke all.
	// The role-cap is enforced in the handler, not at the route, because we
	// need to know the caller's role to decide.
	authed.HandleFunc("GET /api/panel/tokens", s.handleAPITokensList)
	authed.HandleFunc("POST /api/panel/tokens", s.handleAPITokenCreate)
	authed.HandleFunc("DELETE /api/panel/tokens/{id}", s.handleAPITokenDelete)

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

	// Cron — read for any authed user; write is operator (it schedules code to
	// run with elevated privileges).
	authed.HandleFunc("GET /api/cron/{user}", s.handleCronList)
	authed.HandleFunc("POST /api/cron/{user}", s.protected(auth.RoleOperator, s.handleCronAdd))
	authed.HandleFunc("DELETE /api/cron/{user}/{line}", s.protected(auth.RoleOperator, s.handleCronDelete))

	// Firewall — admin-only. A bad rule can lock the panel itself out.
	authed.HandleFunc("GET /api/firewall/status", s.protected(auth.RoleAdmin, s.handleFirewallStatus))
	authed.HandleFunc("POST /api/firewall/rules", s.protected(auth.RoleAdmin, s.handleFirewallAdd))
	authed.HandleFunc("DELETE /api/firewall/rules/{number}", s.protected(auth.RoleAdmin, s.handleFirewallDelete))
	authed.HandleFunc("POST /api/firewall/toggle", s.protected(auth.RoleAdmin, s.handleFirewallToggle))

	// Docker — read for anyone authed; lifecycle / create / remove / exec are operator.
	authed.HandleFunc("GET /api/docker/containers", s.handleDockerContainers)
	authed.HandleFunc("GET /api/docker/images", s.handleDockerImages)
	authed.HandleFunc("GET /api/docker/containers/{id}", s.handleDockerInspect)
	authed.HandleFunc("POST /api/docker/containers", s.protected(auth.RoleOperator, s.handleDockerContainerCreate))
	authed.HandleFunc("DELETE /api/docker/containers/{id}", s.protected(auth.RoleOperator, s.handleDockerContainerRemove))
	authed.HandleFunc("POST /api/docker/containers/{id}/action", s.protected(auth.RoleOperator, s.handleDockerAction))
	authed.HandleFunc("GET /api/docker/containers/{id}/logs/stream", s.handleDockerLogsStream)
	authed.HandleFunc("GET /api/docker/containers/{id}/stats/stream", s.handleDockerStatsStream)
	authed.HandleFunc("GET /api/docker/containers/{id}/exec/ws", s.protected(auth.RoleOperator, s.handleDockerExecWS))

	authed.HandleFunc("GET /api/docker/images/pull", s.protected(auth.RoleOperator, s.handleDockerImagePull))
	authed.HandleFunc("POST /api/docker/images/build", s.protected(auth.RoleOperator, s.handleDockerImageBuild))
	authed.HandleFunc("DELETE /api/docker/images/{ref}", s.protected(auth.RoleOperator, s.handleDockerImageRemove))

	authed.HandleFunc("GET /api/docker/networks", s.handleDockerNetworksList)
	authed.HandleFunc("POST /api/docker/networks", s.protected(auth.RoleOperator, s.handleDockerNetworkCreate))
	authed.HandleFunc("DELETE /api/docker/networks/{id}", s.protected(auth.RoleOperator, s.handleDockerNetworkRemove))

	authed.HandleFunc("GET /api/docker/volumes", s.handleDockerVolumesList)
	authed.HandleFunc("POST /api/docker/volumes", s.protected(auth.RoleOperator, s.handleDockerVolumeCreate))
	authed.HandleFunc("DELETE /api/docker/volumes/{name}", s.protected(auth.RoleOperator, s.handleDockerVolumeRemove))

	authed.HandleFunc("GET /api/docker/info", s.handleDockerInfo)
	authed.HandleFunc("GET /api/docker/df", s.handleDockerDiskUsage)
	authed.HandleFunc("POST /api/docker/prune", s.protected(auth.RoleAdmin, s.handleDockerPrune))

	// Compose stacks — read viewer, write operator. A stack is just a labelled
	// group of containers + their networks/volumes; deploys reuse the existing
	// docker engine client so RBAC and audit-log semantics carry over.
	authed.HandleFunc("GET /api/stacks", s.handleStacksList)
	authed.HandleFunc("GET /api/stacks/{id}", s.handleStackGet)
	authed.HandleFunc("POST /api/stacks", s.protected(auth.RoleOperator, s.handleStackCreate))
	authed.HandleFunc("PUT /api/stacks/{id}", s.protected(auth.RoleOperator, s.handleStackUpdate))
	authed.HandleFunc("POST /api/stacks/{id}/start", s.protected(auth.RoleOperator, s.handleStackStart))
	authed.HandleFunc("POST /api/stacks/{id}/stop", s.protected(auth.RoleOperator, s.handleStackStop))
	authed.HandleFunc("DELETE /api/stacks/{id}", s.protected(auth.RoleOperator, s.handleStackDelete))

	// Network — admin only (a bad change can lock you out of the host).
	authed.HandleFunc("GET /api/network/interfaces", s.protected(auth.RoleAdmin, s.handleNetworkList))
	authed.HandleFunc("GET /api/network/hostname", s.protected(auth.RoleAdmin, s.handleNetworkHostname))
	authed.HandleFunc("POST /api/network/hostname", s.protected(auth.RoleAdmin, s.handleNetworkSetHostname))
	authed.HandleFunc("POST /api/network/interfaces/{name}/static", s.protected(auth.RoleAdmin, s.handleNetworkSetStatic))
	authed.HandleFunc("POST /api/network/interfaces/{name}/dhcp", s.protected(auth.RoleAdmin, s.handleNetworkSetDHCP))
	authed.HandleFunc("POST /api/network/interfaces/{name}/dns", s.protected(auth.RoleAdmin, s.handleNetworkSetDNS))

	// WireGuard — admin only (peer changes route real traffic).
	authed.HandleFunc("GET /api/wireguard/status", s.protected(auth.RoleAdmin, s.handleWGStatus))
	authed.HandleFunc("POST /api/wireguard/peers", s.protected(auth.RoleAdmin, s.handleWGAddPeer))
	authed.HandleFunc("POST /api/wireguard/peers/remove", s.protected(auth.RoleAdmin, s.handleWGRemovePeer))

	// Package management — read viewer+, mutations admin (a bad install can
	// brick a host).
	authed.HandleFunc("GET /api/packages/status", s.handlePackagesStatus)
	authed.HandleFunc("GET /api/packages/search", s.handlePackagesSearch)
	authed.HandleFunc("GET /api/packages/installed", s.protected(auth.RoleOperator, s.handlePackagesInstalled))
	authed.HandleFunc("POST /api/packages/install", s.protected(auth.RoleAdmin, s.handlePackagesInstall))
	authed.HandleFunc("POST /api/packages/remove", s.protected(auth.RoleAdmin, s.handlePackagesRemove))
	authed.HandleFunc("POST /api/packages/upgrade", s.protected(auth.RoleAdmin, s.handlePackagesUpgrade))

	// Backups — list/download is operator+, create/delete is admin (the
	// archive itself contains the panel's whole state).
	authed.HandleFunc("GET /api/backups", s.protected(auth.RoleOperator, s.handleBackupsList))
	authed.HandleFunc("POST /api/backups", s.protected(auth.RoleAdmin, s.handleBackupCreate))
	authed.HandleFunc("GET /api/backups/{id}/download", s.protected(auth.RoleOperator, s.handleBackupDownload))
	authed.HandleFunc("DELETE /api/backups/{id}", s.protected(auth.RoleAdmin, s.handleBackupDelete))

	mux.Handle("/api/auth/logout", s.requireAuth(s.requireCSRF(authed)))
	mux.Handle("/api/", s.requireAuth(s.requireCSRF(authed)))

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
