package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/packages"
	"github.com/Satan1an/webtermin/internal/system"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Same-origin only. The session cookie is SameSite=Strict so a cross-site
	// upgrade attempt from a browser wouldn't even carry credentials, but we
	// still hard-fail any non-matching Origin (and treat missing Origin —
	// typical of non-browser clients that can't carry our cookie either — as
	// not-allowed to keep the WebSocket surface browser-only).
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		return originMatchesHost(origin, r.Host)
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info, err := system.GetInfo()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, info)
}

func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := system.GetMetrics(r.Context(), 0)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) handleSystemMetricsStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send an immediate snapshot.
	if m, err := system.GetMetrics(r.Context(), 0); err == nil {
		_ = conn.WriteJSON(m)
	}
	// Reader loop just to detect close.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-closed:
			return
		case <-ticker.C:
			m, err := system.GetMetrics(r.Context(), 0)
			if err != nil {
				continue
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(m); err != nil {
				return
			}
		}
	}
}

func originMatchesHost(origin, host string) bool {
	// Strip scheme.
	for _, prefix := range []string{"https://", "http://"} {
		if len(origin) > len(prefix) && origin[:len(prefix)] == prefix {
			origin = origin[len(prefix):]
			break
		}
	}
	return origin == host
}

type systemStatus struct {
	Reboot  system.RebootStatus `json:"reboot"`
	Updates packages.Updates    `json:"updates"`
}

// handleSystemStatus feeds the dashboard's "needs attention" cards. Package
// names and versions are not sensitive, so any signed-in role may read them.
func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, systemStatus{
		Reboot:  system.GetRebootStatus(r.Context()),
		Updates: packages.GetUpdates(),
	})
}

// handleSystemReboot reboots the host. The body must say {"confirm":"reboot"}
// so a stray POST (or a replayed request with another body) can't do it.
func (s *Server) handleSystemReboot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Confirm != "reboot" {
		writeJSONError(w, 400, `send {"confirm":"reboot"}`)
		return
	}
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "system.reboot", Outcome: audit.OutcomeOK,
		Detail: strings.Join(system.GetRebootStatus(r.Context()).Reasons, "; "),
	})
	writeJSON(w, 202, map[string]bool{"ok": true})
	// Reply first: once systemd starts the shutdown the connection is gone.
	go func() {
		time.Sleep(2 * time.Second)
		if err := system.Reboot(context.Background()); err != nil {
			s.Log.Error("reboot failed", "err", err)
		}
	}()
}

func (s *Server) handleNotifyStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"enabled":  s.Notify.Enabled(),
		"events":   s.Notify.Events(),
		"disk_pct": s.Notify.DiskPct(),
	})
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	if err := s.Notify.Test(r.Context(), UserFrom(r).Username); err != nil {
		writeJSONError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
