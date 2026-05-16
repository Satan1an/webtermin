package server

import (
	"encoding/json"
	"net/http"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/users"
)

func (s *Server) handleLinuxUsersList(w http.ResponseWriter, r *http.Request) {
	includeSystem := r.URL.Query().Get("system") == "1"
	list, err := users.List(includeSystem)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

type linuxUserCreateReq struct {
	Name     string `json:"name"`
	Gecos    string `json:"gecos"`
	Shell    string `json:"shell"`
	Home     string `json:"home"`
	Password string `json:"password,omitempty"`
}

func (s *Server) handleLinuxUserCreate(w http.ResponseWriter, r *http.Request) {
	var req linuxUserCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := users.Create(users.CreateOpts{
		Name: req.Name, Gecos: req.Gecos, Shell: req.Shell, Home: req.Home,
	}); err != nil {
		s.auditUser(r, "linux.user.create", req.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	if req.Password != "" {
		if err := users.SetPassword(req.Name, req.Password); err != nil {
			s.auditUser(r, "linux.user.create", req.Name, audit.OutcomeError, "password: "+err.Error())
			writeJSONError(w, 500, err.Error())
			return
		}
	}
	s.auditUser(r, "linux.user.create", req.Name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleLinuxUserDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	removeHome := r.URL.Query().Get("remove_home") == "1"
	if err := users.Delete(name, removeHome); err != nil {
		s.auditUser(r, "linux.user.delete", name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "linux.user.delete", name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type passwordReq struct {
	Password string `json:"password"`
}

func (s *Server) handleLinuxUserPassword(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req passwordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := users.SetPassword(name, req.Password); err != nil {
		s.auditUser(r, "linux.user.password", name, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditUser(r, "linux.user.password", name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleLinuxUserKeysList(w http.ResponseWriter, r *http.Request) {
	keys, err := users.ListKeys(r.PathValue("name"))
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, keys)
}

type addKeyReq struct {
	Key string `json:"key"`
}

func (s *Server) handleLinuxUserKeyAdd(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req addKeyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	k, err := users.AddKey(name, req.Key)
	if err != nil {
		s.auditUser(r, "linux.user.key.add", name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "linux.user.key.add", name, audit.OutcomeOK, k.Fingerprint)
	writeJSON(w, 200, k)
}

func (s *Server) handleLinuxUserKeyDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	fp := r.PathValue("fp")
	if err := users.DeleteKey(name, fp); err != nil {
		s.auditUser(r, "linux.user.key.delete", name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "linux.user.key.delete", name, audit.OutcomeOK, fp)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditUser(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
