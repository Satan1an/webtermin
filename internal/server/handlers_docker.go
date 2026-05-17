package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/docker"
)

// dockerOrUnavailable returns the cached client or writes a 503 if Docker
// isn't installed/running on the host. The handler should return immediately
// if !ok.
func (s *Server) dockerOrUnavailable(w http.ResponseWriter) (*docker.Client, bool) {
	c, err := docker.New()
	if err != nil {
		if errors.Is(err, docker.ErrNotAvailable) {
			writeJSONError(w, http.StatusServiceUnavailable, "docker is not installed or its socket is unreachable")
		} else {
			writeJSONError(w, 500, err.Error())
		}
		return nil, false
	}
	return c, true
}

func (s *Server) handleDockerContainers(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	containers, err := c.ListContainers(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, containers)
}

func (s *Server) handleDockerImages(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	images, err := c.ListImages(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, images)
}

type dockerActionReq struct {
	Action string `json:"action"`
}

func (s *Server) handleDockerAction(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !docker.ValidContainerID(id) {
		writeJSONError(w, 400, "invalid container id")
		return
	}
	var req dockerActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !docker.ValidAction(req.Action) {
		writeJSONError(w, 400, "invalid action")
		return
	}
	if err := c.DoAction(r.Context(), id, docker.Action(req.Action)); err != nil {
		s.auditDocker(r, "docker."+req.Action, id, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditDocker(r, "docker."+req.Action, id, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleDockerInspect(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !docker.ValidContainerID(id) {
		writeJSONError(w, 400, "invalid container id")
		return
	}
	out, err := c.Inspect(r.Context(), id)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleDockerLogsStream(w http.ResponseWriter, r *http.Request) {
	c, err := docker.New()
	if err != nil {
		// For WS this needs to upgrade-then-close; simpler: bail before upgrade.
		writeJSONError(w, http.StatusServiceUnavailable, "docker is not available")
		return
	}
	id := r.PathValue("id")
	if !docker.ValidContainerID(id) {
		writeJSONError(w, 400, "invalid container id")
		return
	}
	tail := 200
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	rd, err := c.Logs(r.Context(), id, tail)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer rd.Close()

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

func (s *Server) auditDocker(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
