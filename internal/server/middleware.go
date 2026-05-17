package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/store"
)

// securityHeaders adds defensive headers to every response.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// HSTS — long max-age is safe because we're HTTPS-only.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		// CSP — strict; Monaco needs 'unsafe-eval' for its workers; xterm uses blob:
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-eval' blob:; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' wss: ws:; "+
				"worker-src 'self' blob:; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

// requireAuth accepts either a valid session cookie OR a valid API bearer
// token. Token auth bypasses CSRF (no cookie to ride). Session auth still
// requires the CSRF middleware further down the chain.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try bearer token first — explicit Authorization header beats cookie.
		if tok := bearerToken(r); tok != "" {
			t, user, err := s.lookupAPIToken(tok)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid api token")
				return
			}
			_ = s.Store.TouchAPIToken(t.ID)
			next.ServeHTTP(w, r.WithContext(withTokenAuth(r.Context(), t, user)))
			return
		}

		// Fall back to session cookie.
		c, err := r.Cookie(auth.SessionCookieName)
		if err != nil || c.Value == "" {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		sess, user, err := s.Auth.LookupSession(c.Value)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				auth.ClearSessionCookie(w, true)
			}
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), sess, user)))
	})
}

// bearerToken extracts a `wt_…` API token from the Authorization header.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	v := strings.TrimSpace(h[len(prefix):])
	if !auth.LooksLikeAPIToken(v) {
		return ""
	}
	return v
}

// lookupAPIToken resolves a plaintext token to (token-record, owner-user).
// Token role overrides the owner's current role for the lifetime of the
// request — that's how we enforce "this token is only allowed read-only".
func (s *Server) lookupAPIToken(plaintext string) (*store.APIToken, *store.User, error) {
	hash := auth.HashAPIToken(plaintext)
	t, err := s.Store.GetAPITokenByHash(hash)
	if err != nil {
		return nil, nil, err
	}
	if !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt) {
		return nil, nil, store.ErrNotFound
	}
	user, err := s.Store.GetUser(t.OwnerUserID)
	if err != nil {
		return nil, nil, err
	}
	// Apply the token's scope: clamp the user's effective role to the token's.
	// We make a shallow copy so we don't mutate the cached DB row.
	clamped := *user
	clamped.Role = t.Role
	clamped.IsAdmin = t.Role == string(auth.RoleAdmin)
	return t, &clamped, nil
}

// requireCSRF checks the CSRF token on state-changing requests, but only for
// cookie-backed sessions. API-token clients don't have a session cookie and
// thus aren't vulnerable to cross-site form posts riding their auth.
func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if APITokenFrom(r) != nil {
			next.ServeHTTP(w, r)
			return
		}
		sess := SessionFrom(r)
		if sess == nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		got := r.Header.Get(auth.CSRFHeaderName)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRFToken)) != 1 {
			writeJSONError(w, http.StatusForbidden, "bad CSRF token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// protected wraps a handler with a minimum-role check. Caller must have already
// passed requireSession (so UserFrom is populated). Use it like:
//
//	authed.HandleFunc("POST /api/services/{name}/action",
//	    s.protected(auth.RoleOperator, s.handleServiceAction))
func (s *Server) protected(minRole auth.Role, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r)
		if u == nil || !auth.AtLeast(auth.Role(u.Role), minRole) {
			writeJSONError(w, http.StatusForbidden, "insufficient role")
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
