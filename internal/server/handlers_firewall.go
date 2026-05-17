package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/firewall"
)

func (s *Server) handleFirewallStatus(w http.ResponseWriter, r *http.Request) {
	st, err := firewall.GetStatus()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, st)
}

type firewallAddReq struct {
	Action string `json:"action"` // "allow" | "deny"
	Spec   string `json:"spec"`
}

func (s *Server) handleFirewallAdd(w http.ResponseWriter, r *http.Request) {
	var req firewallAddReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := firewall.Add(req.Action, req.Spec); err != nil {
		s.auditFW(r, "firewall.add", req.Spec, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditFW(r, "firewall.add", req.Spec, audit.OutcomeOK, req.Action)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleFirewallDelete(w http.ResponseWriter, r *http.Request) {
	num, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || num <= 0 {
		writeJSONError(w, 400, "invalid rule number")
		return
	}
	if err := firewall.Delete(num); err != nil {
		s.auditFW(r, "firewall.delete", strconv.Itoa(num), audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditFW(r, "firewall.delete", strconv.Itoa(num), audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type firewallToggleReq struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleFirewallToggle(w http.ResponseWriter, r *http.Request) {
	var req firewallToggleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := firewall.SetEnabled(req.Enabled); err != nil {
		s.auditFW(r, "firewall.toggle", "", audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	state := "disabled"
	if req.Enabled {
		state = "enabled"
	}
	s.auditFW(r, "firewall.toggle", "", audit.OutcomeOK, state)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditFW(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
