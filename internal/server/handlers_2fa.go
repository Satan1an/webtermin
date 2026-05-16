package server

import (
	"encoding/json"
	"net/http"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
)

type enrollResp struct {
	Secret string `json:"secret"`
	OTPURL string `json:"otpauth_url"`
}

// handle2FAEnroll generates a TOTP secret but does NOT activate it until the
// user verifies a code via /verify.
func (s *Server) handle2FAEnroll(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r)
	key, err := auth.GenerateTOTPSecret("webtermin", u.Username)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	// Stash secret in user record but only treat as active after verify.
	// Convention: pending secret is stored prefixed with "pending:".
	if err := s.Store.UpdateUserTOTP(u.ID, "pending:"+key.Secret()); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, enrollResp{Secret: key.Secret(), OTPURL: key.URL()})
}

type verifyReq struct {
	Code string `json:"code"`
}

func (s *Server) handle2FAVerify(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r)
	var req verifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	const pendingPrefix = "pending:"
	if len(u.TOTPSecret) <= len(pendingPrefix) || u.TOTPSecret[:len(pendingPrefix)] != pendingPrefix {
		writeJSONError(w, 400, "no pending 2FA enrollment")
		return
	}
	secret := u.TOTPSecret[len(pendingPrefix):]
	if !auth.ValidateTOTP(secret, req.Code) {
		writeJSONError(w, 400, "invalid code")
		return
	}
	if err := s.Store.UpdateUserTOTP(u.ID, secret); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "auth.2fa.enable", Outcome: audit.OutcomeOK,
	})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r)
	if err := s.Store.UpdateUserTOTP(u.ID, ""); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "auth.2fa.disable", Outcome: audit.OutcomeOK,
	})
	writeJSON(w, 200, map[string]bool{"ok": true})
}
