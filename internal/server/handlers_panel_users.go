package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/store"
)

// panelUserOut is the public shape of a panel user — never includes the hash.
type panelUserOut struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Has2FA    bool      `json:"has_2fa"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toPanelUser(u *store.User) panelUserOut {
	return panelUserOut{
		ID: u.ID, Username: u.Username, Role: u.Role,
		Has2FA: u.TOTPSecret != "",
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func (s *Server) handlePanelUsersList(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	out := make([]panelUserOut, 0, len(users))
	for _, u := range users {
		out = append(out, toPanelUser(u))
	}
	writeJSON(w, 200, out)
}

type panelUserCreateReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handlePanelUserCreate(w http.ResponseWriter, r *http.Request) {
	var req panelUserCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if !validUsername(req.Username) {
		writeJSONError(w, 400, "invalid username")
		return
	}
	if len(req.Password) < 10 {
		writeJSONError(w, 400, "password must be at least 10 characters")
		return
	}
	if !auth.ValidRole(req.Role) {
		writeJSONError(w, 400, "invalid role")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSONError(w, 500, "hash failed")
		return
	}
	created, err := s.Store.CreateUser(req.Username, hash, "", req.Role, req.Role == string(auth.RoleAdmin))
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditPanelUser(r, "panel.user.create", req.Username, audit.OutcomeOK, "role="+req.Role)
	writeJSON(w, 200, toPanelUser(created))
}

func (s *Server) handlePanelUserDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	target, err := s.Store.GetUser(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, 404, "user not found")
			return
		}
		writeJSONError(w, 500, err.Error())
		return
	}
	caller := UserFrom(r)
	if target.ID == caller.ID {
		writeJSONError(w, 400, "cannot delete your own account")
		return
	}
	if err := s.guardLastAdmin(target); err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	if err := s.Store.DeleteUser(target.ID); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditPanelUser(r, "panel.user.delete", target.Username, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type panelUserRoleReq struct {
	Role string `json:"role"`
}

func (s *Server) handlePanelUserSetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	var req panelUserRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if !auth.ValidRole(req.Role) {
		writeJSONError(w, 400, "invalid role")
		return
	}
	target, err := s.Store.GetUser(id)
	if err != nil {
		writeJSONError(w, 404, "user not found")
		return
	}
	// If we're demoting away from admin, make sure another admin exists.
	if target.Role == string(auth.RoleAdmin) && req.Role != string(auth.RoleAdmin) {
		if err := s.guardLastAdmin(target); err != nil {
			writeJSONError(w, 400, err.Error())
			return
		}
	}
	if err := s.Store.UpdateUserRole(target.ID, req.Role); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditPanelUser(r, "panel.user.role", target.Username, audit.OutcomeOK,
		target.Role+" -> "+req.Role)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type panelUserPasswordReq struct {
	Password string `json:"password"`
}

func (s *Server) handlePanelUserSetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	var req panelUserPasswordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if len(req.Password) < 10 {
		writeJSONError(w, 400, "password must be at least 10 characters")
		return
	}
	target, err := s.Store.GetUser(id)
	if err != nil {
		writeJSONError(w, 404, "user not found")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSONError(w, 500, "hash failed")
		return
	}
	if err := s.Store.UpdateUserPassword(target.ID, hash); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditPanelUser(r, "panel.user.password", target.Username, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// guardLastAdmin returns an error if removing/demoting `target` would leave
// the panel with zero admins. Prevents the system from becoming unrecoverable.
func (s *Server) guardLastAdmin(target *store.User) error {
	if target.Role != string(auth.RoleAdmin) {
		return nil
	}
	users, err := s.Store.ListUsers()
	if err != nil {
		return err
	}
	admins := 0
	for _, u := range users {
		if u.Role == string(auth.RoleAdmin) {
			admins++
		}
	}
	if admins <= 1 {
		return errors.New("cannot remove the last admin")
	}
	return nil
}

func pathInt64(r *http.Request, name string) (int64, bool) {
	v := r.PathValue(name)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func (s *Server) auditPanelUser(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
