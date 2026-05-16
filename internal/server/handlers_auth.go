package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/store"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTP     string `json:"totp,omitempty"`
}

type sessionInfo struct {
	User      string `json:"user"`
	IsAdmin   bool   `json:"is_admin"` // kept for v0.1 clients; derived from role
	Role      string `json:"role"`
	CSRFToken string `json:"csrf_token"`
	Has2FA    bool   `json:"has_2fa"`
}

func toSessionInfo(u *store.User, csrf string) sessionInfo {
	return sessionInfo{
		User:      u.Username,
		IsAdmin:   u.Role == string(auth.RoleAdmin),
		Role:      u.Role,
		CSRFToken: csrf,
		Has2FA:    u.TOTPSecret != "",
	}
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.Store.CountUsers()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	resp := map[string]any{
		"needs_setup": count == 0,
	}
	// If a session cookie is set and valid, surface user info.
	if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
		if sess, u, err := s.Auth.LookupSession(c.Value); err == nil {
			resp["user"] = toSessionInfo(u, sess.CSRFToken)
		}
	}
	writeJSON(w, 200, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	ip := auth.ClientIP(r)
	ua := r.UserAgent()

	res, err := s.Auth.Login(req.Username, req.Password, req.TOTP, ip, ua)
	if err != nil {
		s.Audit.Write(audit.Event{
			Username: req.Username, IP: ip, Action: "auth.login",
			Outcome: audit.OutcomeDenied, Detail: err.Error(),
		})
		switch {
		case errors.Is(err, auth.ErrLockedOut):
			// Surface the lockout window so well-behaved clients back off
			// instead of hammering the endpoint.
			w.Header().Set("Retry-After", strconv.Itoa(int(s.Cfg.Security.Lockout().Seconds())))
			writeJSONError(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, auth.ErrTOTPRequired):
			writeJSON(w, 401, map[string]string{"error": err.Error(), "code": "totp_required"})
		default:
			writeJSONError(w, 401, "invalid credentials")
		}
		return
	}
	auth.IssueSessionCookie(w, res.Session.ID, s.Cfg.Security.SessionTTL(), true)
	uid := res.User.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: res.User.Username, IP: ip, Action: "auth.login",
		Outcome: audit.OutcomeOK,
	})
	writeJSON(w, 200, toSessionInfo(res.User, res.Session.CSRFToken))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	u := UserFrom(r)
	if sess != nil {
		_ = s.Auth.Logout(sess.ID)
		s.Audit.Write(audit.Event{
			UserID: &u.ID, Username: u.Username, IP: auth.ClientIP(r),
			Action: "auth.logout", Outcome: audit.OutcomeOK,
		})
	}
	auth.ClearSessionCookie(w, true)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	u := UserFrom(r)
	writeJSON(w, 200, toSessionInfo(u, sess.CSRFToken))
}

type setupReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleFirstRunSetup(w http.ResponseWriter, r *http.Request) {
	count, err := s.Store.CountUsers()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	if count > 0 {
		writeJSONError(w, http.StatusConflict, "already configured")
		return
	}
	var req setupReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !validUsername(req.Username) {
		writeJSONError(w, 400, "invalid username")
		return
	}
	if len(req.Password) < 10 {
		writeJSONError(w, 400, "password must be at least 10 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSONError(w, 500, "hash failed")
		return
	}
	u, err := s.Store.CreateUser(req.Username, hash, "", string(auth.RoleAdmin), true)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.Audit.Write(audit.Event{
		UserID: &u.ID, Username: u.Username, IP: auth.ClientIP(r),
		Action: "auth.setup", Outcome: audit.OutcomeOK, Detail: "first-run admin",
	})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleAuditList(w http.ResponseWriter, r *http.Request) {
	entries, err := s.Store.ListAudit(200)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, entries)
}

func validUsername(u string) bool {
	if len(u) < 1 || len(u) > 32 {
		return false
	}
	for _, r := range u {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}
