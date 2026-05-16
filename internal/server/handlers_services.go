package server

import (
	"bufio"
	"encoding/json"
	"net/http"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/systemd"
)

func (s *Server) handleServicesList(w http.ResponseWriter, r *http.Request) {
	t := r.URL.Query().Get("type")
	if t == "" {
		t = "service"
	}
	units, err := systemd.List(r.Context(), t)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, units)
}

type serviceActionReq struct {
	Action string `json:"action"`
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !systemd.ValidUnitName(name) {
		writeJSONError(w, 400, "invalid unit name")
		return
	}
	var req serviceActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !systemd.ValidAction(req.Action) {
		writeJSONError(w, 400, "invalid action")
		return
	}
	u := UserFrom(r)
	uid := u.ID
	result, err := systemd.Do(r.Context(), name, systemd.Action(req.Action))
	outcome := audit.OutcomeOK
	detail := result
	if err != nil {
		outcome = audit.OutcomeError
		detail = err.Error()
	}
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action:  "systemd." + req.Action,
		Target:  name,
		Outcome: outcome, Detail: detail,
	})
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"result": result})
}

func (s *Server) handleServiceJournalStream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !systemd.ValidUnitName(name) {
		writeJSONError(w, 400, "invalid unit name")
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	rd, err := systemd.TailJournal(r.Context(), name, 200)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer rd.Close()

	// Detect client-side close.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	scanner := bufio.NewScanner(rd)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		default:
		}
		if err := conn.WriteJSON(map[string]string{"line": scanner.Text()}); err != nil {
			return
		}
	}
}
