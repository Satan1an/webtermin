package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/config"
	"github.com/Satan1an/webtermin/internal/store"
)

const (
	SessionCookieName = "webtermin_session"
	CSRFHeaderName    = "X-CSRF-Token"
	sessionTokenBytes = 32
	csrfTokenBytes    = 32
)

var (
	ErrLockedOut      = errors.New("too many failed attempts, try again later")
	ErrBadCredentials = errors.New("invalid username or password")
	ErrTOTPRequired   = errors.New("2FA code required")
	ErrTOTPInvalid    = errors.New("invalid 2FA code")
)

type Service struct {
	Store *store.Store
	Cfg   *config.Config
}

func New(s *store.Store, c *config.Config) *Service { return &Service{Store: s, Cfg: c} }

type LoginResult struct {
	Session *store.Session
	User    *store.User
}

// Login authenticates a user and creates a session. ip and ua are recorded.
// If require2FA is set or the user has TOTP configured, totpCode must be valid.
func (a *Service) Login(username, password, totpCode, ip, ua string) (*LoginResult, error) {
	failures, err := a.Store.CountRecentFailures(ip, time.Now().Add(-a.Cfg.Security.Lockout()))
	if err != nil {
		return nil, err
	}
	if a.Cfg.Security.MaxLoginAttempts > 0 && failures >= a.Cfg.Security.MaxLoginAttempts {
		return nil, ErrLockedOut
	}

	user, err := a.Store.GetUserByName(username)
	if err != nil {
		_ = a.Store.RecordLoginAttempt(ip, username, false)
		// constant-ish-time: still hash a dummy to avoid trivial timing leak
		_, _ = HashPassword("dummy-to-equalize-timing")
		return nil, ErrBadCredentials
	}
	if err := VerifyPassword(password, user.PWHash); err != nil {
		_ = a.Store.RecordLoginAttempt(ip, username, false)
		return nil, ErrBadCredentials
	}

	if user.TOTPSecret != "" || a.Cfg.Security.Require2FA {
		if user.TOTPSecret == "" {
			// require2FA is on but this user hasn't enrolled yet — surface as
			// "totp_required" so the UI can route them to /2fa/enroll.
			return nil, ErrTOTPRequired
		}
		if totpCode == "" {
			return nil, ErrTOTPRequired
		}
		if !ValidateTOTP(user.TOTPSecret, totpCode) {
			_ = a.Store.RecordLoginAttempt(ip, username, false)
			return nil, ErrTOTPInvalid
		}
	}

	sid := RandomToken(sessionTokenBytes)
	csrf := RandomToken(csrfTokenBytes)
	sess, err := a.Store.CreateSession(sid, user.ID, csrf, a.Cfg.Security.SessionTTL(), ip, ua)
	if err != nil {
		return nil, err
	}
	_ = a.Store.RecordLoginAttempt(ip, username, true)
	return &LoginResult{Session: sess, User: user}, nil
}

func (a *Service) Logout(sessionID string) error {
	return a.Store.DeleteSession(sessionID)
}

// LookupSession returns (session, user) for a valid, non-expired cookie value.
func (a *Service) LookupSession(sid string) (*store.Session, *store.User, error) {
	sess, err := a.Store.GetSession(sid)
	if err != nil {
		return nil, nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = a.Store.DeleteSession(sid)
		return nil, nil, store.ErrNotFound
	}
	u, err := a.Store.GetUser(sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	_ = a.Store.TouchSession(sid)
	return sess, u, nil
}

// IssueSessionCookie sets the session cookie with safe attributes.
func IssueSessionCookie(w http.ResponseWriter, value string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClientIP extracts the remote IP, stripping the port.
func ClientIP(r *http.Request) string {
	// Honour X-Forwarded-For only if explicitly trusted (not by default).
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
