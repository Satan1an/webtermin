package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/cron"
	"github.com/Satan1an/webtermin/internal/users"
)

func (s *Server) handleCronList(w http.ResponseWriter, r *http.Request) {
	user := r.PathValue("user")
	if !users.ValidName(user) {
		writeJSONError(w, 400, "invalid user")
		return
	}
	entries, err := cron.List(user)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, entries)
}

type cronAddReq struct {
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Comment  string `json:"comment,omitempty"`
}

func (s *Server) handleCronAdd(w http.ResponseWriter, r *http.Request) {
	user := r.PathValue("user")
	if !users.ValidName(user) {
		writeJSONError(w, 400, "invalid user")
		return
	}
	var req cronAddReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	e := cron.Entry{Schedule: req.Schedule, Command: req.Command, Comment: req.Comment}
	if err := cron.Add(user, e); err != nil {
		s.auditCron(r, "cron.add", user, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditCron(r, "cron.add", user, audit.OutcomeOK, req.Schedule+" "+req.Command)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleCronDelete(w http.ResponseWriter, r *http.Request) {
	user := r.PathValue("user")
	if !users.ValidName(user) {
		writeJSONError(w, 400, "invalid user")
		return
	}
	lineNo, err := strconv.Atoi(r.PathValue("line"))
	if err != nil || lineNo <= 0 {
		writeJSONError(w, 400, "invalid line number")
		return
	}
	if err := cron.DeleteLine(user, lineNo); err != nil {
		s.auditCron(r, "cron.delete", user, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditCron(r, "cron.delete", user, audit.OutcomeOK, "line="+strconv.Itoa(lineNo))
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditCron(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
