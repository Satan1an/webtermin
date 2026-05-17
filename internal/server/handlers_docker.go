package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

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

// --- Containers: create / remove / stats stream / exec console ----------

func (s *Server) handleDockerContainerCreate(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	var spec docker.CreateContainerSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	id, err := c.CreateContainer(r.Context(), spec)
	if err != nil {
		s.auditDocker(r, "docker.create", spec.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditDocker(r, "docker.create", spec.Name, audit.OutcomeOK, "image="+spec.Image)
	writeJSON(w, 200, map[string]string{"id": id})
}

func (s *Server) handleDockerContainerRemove(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !docker.ValidContainerID(id) {
		writeJSONError(w, 400, "invalid container id")
		return
	}
	force := r.URL.Query().Get("force") == "1"
	if err := c.RemoveContainer(r.Context(), id, force); err != nil {
		s.auditDocker(r, "docker.remove", id, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditDocker(r, "docker.remove", id, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleDockerStatsStream(w http.ResponseWriter, r *http.Request) {
	c, err := docker.New()
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "docker is not available")
		return
	}
	id := r.PathValue("id")
	if !docker.ValidContainerID(id) {
		writeJSONError(w, 400, "invalid container id")
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	rd, err := c.Stats(r.Context(), id)
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

	dec := json.NewDecoder(rd)
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			return
		}
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		default:
		}
		if err := conn.WriteJSON(raw); err != nil {
			return
		}
	}
}

type execClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Cols int    `json:"cols,omitempty"`
}

func (s *Server) handleDockerExecWS(w http.ResponseWriter, r *http.Request) {
	c, err := docker.New()
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "docker is not available")
		return
	}
	id := r.PathValue("id")
	if !docker.ValidContainerID(id) {
		writeJSONError(w, 400, "invalid container id")
		return
	}
	shell := r.URL.Query().Get("shell")
	if shell == "" {
		shell = "/bin/sh"
	}
	execID, err := c.ExecCreate(r.Context(), id, []string{shell}, true)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	stream, err := c.ExecStart(r.Context(), execID, true)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		stream.Close()
		return
	}
	defer wsConn.Close()
	defer stream.Close()

	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "docker.exec.open", Target: id,
		Outcome: audit.OutcomeOK, Detail: "shell=" + shell,
	})

	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		buf := make([]byte, 8192)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				_ = wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if werr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				_ = wsConn.WriteJSON(map[string]string{"type": "closed", "detail": err.Error()})
				return
			}
		}
	}()

	for {
		mt, data, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		switch mt {
		case websocket.BinaryMessage:
			_, _ = stream.Write(data)
		case websocket.TextMessage:
			var msg execClientMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "data":
				_, _ = stream.Write([]byte(msg.Data))
			case "resize":
				_ = c.ExecResize(r.Context(), execID, msg.Rows, msg.Cols)
			}
		}
	}
	<-streamDone
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "docker.exec.close", Target: id, Outcome: audit.OutcomeOK,
	})
}

// --- Images: pull (streamed) / remove ----------------------------------

type imagePullReq struct {
	Image string `json:"image"`
}

func (s *Server) handleDockerImagePull(w http.ResponseWriter, r *http.Request) {
	c, err := docker.New()
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "docker is not available")
		return
	}
	var req imagePullReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if !docker.ValidImageRef(req.Image) {
		writeJSONError(w, 400, "invalid image reference")
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	rd, err := c.PullImage(r.Context(), req.Image)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}
	defer rd.Close()

	dec := json.NewDecoder(rd)
	for {
		var ev map[string]any
		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				_ = conn.WriteJSON(map[string]string{"type": "done"})
			}
			return
		}
		if err := conn.WriteJSON(ev); err != nil {
			return
		}
	}
}

func (s *Server) handleDockerImageRemove(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	ref := r.PathValue("ref")
	force := r.URL.Query().Get("force") == "1"
	if err := c.RemoveImage(r.Context(), ref, force); err != nil {
		s.auditDocker(r, "docker.image.remove", ref, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditDocker(r, "docker.image.remove", ref, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// --- Networks ----------------------------------------------------------

func (s *Server) handleDockerNetworksList(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	nets, err := c.ListNetworks(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, nets)
}

func (s *Server) handleDockerNetworkCreate(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	var spec docker.CreateNetworkSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	id, err := c.CreateNetwork(r.Context(), spec)
	if err != nil {
		s.auditDocker(r, "docker.network.create", spec.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditDocker(r, "docker.network.create", spec.Name, audit.OutcomeOK, "driver="+spec.Driver)
	writeJSON(w, 200, map[string]string{"id": id})
}

func (s *Server) handleDockerNetworkRemove(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := c.RemoveNetwork(r.Context(), id); err != nil {
		s.auditDocker(r, "docker.network.remove", id, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditDocker(r, "docker.network.remove", id, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// --- Volumes -----------------------------------------------------------

func (s *Server) handleDockerVolumesList(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	vols, err := c.ListVolumes(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, vols)
}

func (s *Server) handleDockerVolumeCreate(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	var spec docker.CreateVolumeSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	v, err := c.CreateVolume(r.Context(), spec)
	if err != nil {
		s.auditDocker(r, "docker.volume.create", spec.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditDocker(r, "docker.volume.create", spec.Name, audit.OutcomeOK, "")
	writeJSON(w, 200, v)
}

func (s *Server) handleDockerVolumeRemove(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	name := r.PathValue("name")
	force := r.URL.Query().Get("force") == "1"
	if err := c.RemoveVolume(r.Context(), name, force); err != nil {
		s.auditDocker(r, "docker.volume.remove", name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditDocker(r, "docker.volume.remove", name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// --- System ------------------------------------------------------------

func (s *Server) handleDockerInfo(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	info, err := c.Info(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, info)
}

func (s *Server) handleDockerDiskUsage(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	df, err := c.DiskUsage(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, df)
}

type pruneReq struct {
	Target string `json:"target"`
}

func (s *Server) handleDockerPrune(w http.ResponseWriter, r *http.Request) {
	c, ok := s.dockerOrUnavailable(w)
	if !ok {
		return
	}
	var req pruneReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !docker.ValidPruneTarget(req.Target) {
		writeJSONError(w, 400, "invalid prune target")
		return
	}
	out, err := c.Prune(r.Context(), docker.PruneTarget(req.Target))
	if err != nil {
		s.auditDocker(r, "docker.prune", req.Target, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditDocker(r, "docker.prune", req.Target, audit.OutcomeOK, "")
	writeJSON(w, 200, out)
}

// Used in pruning/strconv imports; if compiler complains, we still reference them.
var _ = strconv.Atoi
