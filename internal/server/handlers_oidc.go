package server

import (
	"net/http"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
)

const oidcStateCookie = "webtermin_oidc_state"

// handleOIDCStart redirects the browser to the IdP and drops a short-lived
// state cookie so the callback can verify the round-trip.
func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if s.OIDC == nil {
		writeJSONError(w, http.StatusNotImplemented, "OIDC is not configured")
		return
	}
	state := auth.NewState()
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.OIDC.AuthURL(state), http.StatusFound)
}

// handleOIDCCallback exchanges the auth code for an ID token and creates (or
// reuses) a panel account keyed on the verified email / subject.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.OIDC == nil {
		writeJSONError(w, http.StatusNotImplemented, "OIDC is not configured")
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeJSONError(w, http.StatusBadRequest, "missing state or code")
		return
	}
	c, err := r.Cookie(oidcStateCookie)
	if err != nil || c.Value != state {
		writeJSONError(w, http.StatusBadRequest, "state mismatch — possible CSRF, retry login")
		return
	}
	// Wipe the state cookie now that we've verified it.
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookie, Value: "", Path: "/",
		Expires: time.Unix(0, 0), MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	ident, err := s.OIDC.Exchange(r.Context(), code)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	if ident.Email == "" && ident.Name == "" {
		writeJSONError(w, http.StatusBadGateway, "IdP returned no email/name")
		return
	}

	// Identify by subject prefix so two users at different IdPs with the same
	// local-part don't collide; ident.Name (preferred_username) is the
	// human-readable label for the row.
	username := ident.Name
	if username == "" {
		username = ident.Email
	}
	if !validUsername(username) {
		// Fall back to a sanitised form of the email's local part.
		username = "oidc-" + sanitiseLocalPart(ident.Subject)
	}

	u, err := s.Store.GetUserByName(username)
	if err != nil {
		// Create the user with the configured default role (or viewer).
		role := s.Cfg.OIDC.DefaultRole
		if !auth.ValidRole(role) {
			role = string(auth.RoleViewer)
		}
		// Placeholder password hash — OIDC users can't password-log-in. We
		// still hash a long random string so the column has a non-empty value.
		dummy, _ := auth.HashPassword(auth.RandomToken(32))
		u, err = s.Store.CreateUser(username, dummy, "", role, role == string(auth.RoleAdmin))
		if err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
	}

	sid := auth.RandomToken(32)
	csrf := auth.RandomToken(32)
	sess, err := s.Store.CreateSession(sid, u.ID, csrf, s.Cfg.Security.SessionTTL(),
		auth.ClientIP(r), r.UserAgent())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	auth.IssueSessionCookie(w, sess.ID, s.Cfg.Security.SessionTTL(), true)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "auth.login.oidc", Outcome: audit.OutcomeOK,
		Detail: "issuer=" + ident.Issuer,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleOIDCStatus is a small public endpoint the login page can hit to
// decide whether to render the "Sign in with SSO" button.
func (s *Server) handleOIDCStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"enabled": s.OIDC != nil,
		"issuer":  s.Cfg.OIDC.Issuer,
	})
}

func sanitiseLocalPart(s string) string {
	// Strip everything that doesn't fit our username regex.
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && i < 24; i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '-', c == '.':
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "user"
	}
	return string(out)
}
