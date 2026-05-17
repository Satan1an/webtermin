package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/store"
)

// apiTokenOut is the public shape of a token — never includes the hash or the
// plaintext (which is only returned once at creation).
type apiTokenOut struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`
	OwnerUserID int64     `json:"owner_user_id"`
	OwnerName   string    `json:"owner_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

func toAPITokenOut(t *store.APIToken, ownerName string) apiTokenOut {
	out := apiTokenOut{
		ID:          t.ID,
		Name:        t.Name,
		Role:        t.Role,
		OwnerUserID: t.OwnerUserID,
		OwnerName:   ownerName,
		CreatedAt:   t.CreatedAt,
	}
	if !t.LastUsedAt.IsZero() {
		out.LastUsedAt = t.LastUsedAt
	}
	if !t.ExpiresAt.IsZero() {
		out.ExpiresAt = t.ExpiresAt
	}
	return out
}

func (s *Server) handleAPITokensList(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r)
	// Admins see all tokens; everyone else sees only their own.
	var ownerFilter int64
	if u.Role != string(auth.RoleAdmin) {
		ownerFilter = u.ID
	}
	tokens, err := s.Store.ListAPITokens(ownerFilter)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	out := make([]apiTokenOut, 0, len(tokens))
	for _, t := range tokens {
		name := ""
		if owner, err := s.Store.GetUser(t.OwnerUserID); err == nil {
			name = owner.Username
		}
		out = append(out, toAPITokenOut(t, name))
	}
	writeJSON(w, 200, out)
}

type apiTokenCreateReq struct {
	Name          string `json:"name"`
	Role          string `json:"role"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
}

type apiTokenCreateResp struct {
	Token apiTokenOut `json:"token"`
	// Plaintext is returned exactly once — the user must copy it now.
	Plaintext string `json:"plaintext"`
}

func (s *Server) handleAPITokenCreate(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r)
	var req apiTokenCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 64 {
		writeJSONError(w, 400, "token name is required (1–64 chars)")
		return
	}
	if !auth.ValidRole(req.Role) {
		writeJSONError(w, 400, "invalid role")
		return
	}
	// Role-cap: a user can't issue a token with more privilege than they have.
	if !auth.AtLeast(auth.Role(u.Role), auth.Role(req.Role)) {
		writeJSONError(w, http.StatusForbidden, "cannot issue a token with higher role than yourself")
		return
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > 365*5 {
		writeJSONError(w, 400, "expires_in_days must be between 0 and 1825")
		return
	}

	plaintext, hash := auth.NewAPIToken()
	var exp time.Time
	if req.ExpiresInDays > 0 {
		exp = time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
	}
	created, err := s.Store.CreateAPIToken(req.Name, hash, req.Role, u.ID, exp)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "panel.token.create", Target: req.Name,
		Outcome: audit.OutcomeOK,
		Detail:  "role=" + req.Role,
	})
	writeJSON(w, 200, apiTokenCreateResp{
		Token:     toAPITokenOut(created, u.Username),
		Plaintext: plaintext,
	})
}

func (s *Server) handleAPITokenDelete(w http.ResponseWriter, r *http.Request) {
	caller := UserFrom(r)
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	t, err := s.Store.GetAPIToken(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, 404, "token not found")
			return
		}
		writeJSONError(w, 500, err.Error())
		return
	}
	// Owner can revoke own; admin can revoke any.
	if t.OwnerUserID != caller.ID && caller.Role != string(auth.RoleAdmin) {
		writeJSONError(w, http.StatusForbidden, "not your token")
		return
	}
	if err := s.Store.DeleteAPIToken(t.ID); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	cid := caller.ID
	s.Audit.Write(audit.Event{
		UserID: &cid, Username: caller.Username, IP: auth.ClientIP(r),
		Action: "panel.token.revoke", Target: t.Name,
		Outcome: audit.OutcomeOK,
	})
	writeJSON(w, 200, map[string]bool{"ok": true})
}
