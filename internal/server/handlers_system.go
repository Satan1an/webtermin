package server

import (
	"net/http"
	"time"

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
