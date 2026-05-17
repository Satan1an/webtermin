package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/compose"
	"github.com/Satan1an/webtermin/internal/docker"
	"github.com/Satan1an/webtermin/internal/store"
)

type stackOut struct {
	ID          int64              `json:"id"`
	Name        string             `json:"name"`
	Compose     string             `json:"compose"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	Containers  []docker.Container `json:"containers,omitempty"`
	Status      string             `json:"status,omitempty"` // running | partial | stopped | empty
	ServiceList []string           `json:"services,omitempty"`
}

func toStackOut(st *store.Stack, mgr *compose.Manager, r *http.Request) stackOut {
	out := stackOut{
		ID: st.ID, Name: st.Name, Compose: st.Compose,
		CreatedAt: st.CreatedAt, UpdatedAt: st.UpdatedAt,
	}
	if mgr != nil {
		if cs, err := mgr.ListContainers(r.Context(), st.Name); err == nil {
			out.Containers = cs
			out.Status = summariseState(cs)
		}
	}
	if f, err := compose.Parse(st.Compose); err == nil {
		for name := range f.Services {
			out.ServiceList = append(out.ServiceList, name)
		}
	}
	return out
}

func summariseState(cs []docker.Container) string {
	if len(cs) == 0 {
		return "empty"
	}
	running := 0
	for _, c := range cs {
		if c.State == "running" {
			running++
		}
	}
	if running == len(cs) {
		return "running"
	}
	if running == 0 {
		return "stopped"
	}
	return "partial"
}

func (s *Server) dockerMgrOr503(w http.ResponseWriter) (*compose.Manager, bool) {
	c, err := docker.New()
	if err != nil {
		if errors.Is(err, docker.ErrNotAvailable) {
			writeJSONError(w, http.StatusServiceUnavailable, "docker is not installed or its socket is unreachable")
		} else {
			writeJSONError(w, 500, err.Error())
		}
		return nil, false
	}
	return compose.NewManager(c), true
}

func (s *Server) handleStacksList(w http.ResponseWriter, r *http.Request) {
	stacks, err := s.Store.ListStacks()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	mgr, _ := docker.New()
	var m *compose.Manager
	if mgr != nil {
		m = compose.NewManager(mgr)
	}
	out := make([]stackOut, 0, len(stacks))
	for _, st := range stacks {
		out = append(out, toStackOut(st, m, r))
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleStackGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	st, err := s.Store.GetStack(id)
	if err != nil {
		writeJSONError(w, 404, "stack not found")
		return
	}
	mgr, _ := s.dockerMgrOptional()
	writeJSON(w, 200, toStackOut(st, mgr, r))
}

// dockerMgrOptional returns a manager if Docker is up, nil otherwise. We don't
// 503 on the read path — listing stacks must work even when the daemon is down.
func (s *Server) dockerMgrOptional() (*compose.Manager, bool) {
	c, err := docker.New()
	if err != nil {
		return nil, false
	}
	return compose.NewManager(c), true
}

type stackCreateReq struct {
	Name    string `json:"name"`
	Compose string `json:"compose"`
}

func (s *Server) handleStackCreate(w http.ResponseWriter, r *http.Request) {
	var req stackCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if !compose.ValidStackName(req.Name) {
		writeJSONError(w, 400, "invalid stack name (lowercase a–z, 0–9, _ or -, 1–32 chars)")
		return
	}
	file, err := compose.Parse(req.Compose)
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	if _, err := s.Store.GetStackByName(req.Name); err == nil {
		writeJSONError(w, http.StatusConflict, "stack already exists")
		return
	}
	mgr, ok := s.dockerMgrOr503(w)
	if !ok {
		return
	}
	if _, err := mgr.Deploy(r.Context(), req.Name, file); err != nil {
		s.auditStack(r, "stacks.deploy", req.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	st, err := s.Store.CreateStack(req.Name, req.Compose)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditStack(r, "stacks.deploy", req.Name, audit.OutcomeOK, "services="+joinKeys(file.Services))
	writeJSON(w, 200, toStackOut(st, mgr, r))
}

type stackUpdateReq struct {
	Compose string `json:"compose"`
}

func (s *Server) handleStackUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	st, err := s.Store.GetStack(id)
	if err != nil {
		writeJSONError(w, 404, "stack not found")
		return
	}
	var req stackUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	file, err := compose.Parse(req.Compose)
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	mgr, ok := s.dockerMgrOr503(w)
	if !ok {
		return
	}
	if _, err := mgr.Deploy(r.Context(), st.Name, file); err != nil {
		s.auditStack(r, "stacks.update", st.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	if err := s.Store.UpdateStackCompose(st.ID, req.Compose); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditStack(r, "stacks.update", st.Name, audit.OutcomeOK, "services="+joinKeys(file.Services))
	st2, _ := s.Store.GetStack(id)
	writeJSON(w, 200, toStackOut(st2, mgr, r))
}

func (s *Server) handleStackStart(w http.ResponseWriter, r *http.Request) {
	st, mgr, ok := s.resolveStack(w, r)
	if !ok {
		return
	}
	if err := mgr.Start(r.Context(), st.Name); err != nil {
		s.auditStack(r, "stacks.start", st.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditStack(r, "stacks.start", st.Name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleStackStop(w http.ResponseWriter, r *http.Request) {
	st, mgr, ok := s.resolveStack(w, r)
	if !ok {
		return
	}
	if err := mgr.Stop(r.Context(), st.Name); err != nil {
		s.auditStack(r, "stacks.stop", st.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditStack(r, "stacks.stop", st.Name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleStackDelete(w http.ResponseWriter, r *http.Request) {
	st, mgr, ok := s.resolveStack(w, r)
	if !ok {
		return
	}
	removeData := r.URL.Query().Get("remove_data") == "1"
	file, err := compose.Parse(st.Compose)
	if err != nil {
		// Stored compose is malformed — fall back to just removing containers
		// by label, leaving networks/volumes alone.
		if err := mgr.RemoveContainers(r.Context(), st.Name, true); err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
	} else if err := mgr.RemoveStack(r.Context(), st.Name, file, removeData); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	if err := s.Store.DeleteStack(st.ID); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditStack(r, "stacks.delete", st.Name, audit.OutcomeOK,
		"remove_data="+boolStr(removeData))
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) resolveStack(w http.ResponseWriter, r *http.Request) (*store.Stack, *compose.Manager, bool) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return nil, nil, false
	}
	st, err := s.Store.GetStack(id)
	if err != nil {
		writeJSONError(w, 404, "stack not found")
		return nil, nil, false
	}
	mgr, ok := s.dockerMgrOr503(w)
	if !ok {
		return nil, nil, false
	}
	return st, mgr, true
}

func (s *Server) auditStack(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}

func joinKeys[V any](m map[string]V) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	out := keys[0]
	for _, k := range keys[1:] {
		out += "," + k
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
