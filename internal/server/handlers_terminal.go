package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/pty"
)

// Client → server protocol (text frames as JSON):
//   {"type":"data","data":"..."}      // keystrokes
//   {"type":"resize","rows":24,"cols":80}
//
// Server → client (binary or text):
//   binary frames: raw PTY output (UTF-8)
//   text frames:   JSON {"type":"closed","detail":"..."} on EOF
type wsClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
}

func (s *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r)
	rows, cols := parseSize(r)
	// Try the panel user as a Linux user first; if that account doesn't exist,
	// fall back to the process user so the terminal still works.
	session, err := pty.Start(u.Username, s.Cfg.Terminal.DefaultShell, rows, cols)
	if err != nil {
		session, err = pty.Start("", s.Cfg.Terminal.DefaultShell, rows, cols)
		if err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = session.Close()
		return
	}
	defer conn.Close()
	defer session.Close()

	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "terminal.open", Outcome: audit.OutcomeOK,
	})

	idle := s.Cfg.Security.PTYIdleTimeout()
	conn.SetReadDeadline(time.Now().Add(idle))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(idle))
		return nil
	})

	// Ping loop to detect dead conns.
	stopPing := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-t.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()
	defer close(stopPing)

	// PTY → WS pump
	ptyDone := make(chan struct{})
	go func() {
		defer close(ptyDone)
		buf := make([]byte, 8192)
		for {
			n, err := session.Read(buf)
			if n > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				_ = conn.WriteJSON(map[string]string{"type": "closed", "detail": err.Error()})
				return
			}
		}
	}()

	// WS → PTY pump
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(idle))
		switch mt {
		case websocket.BinaryMessage:
			_, _ = session.Write(data)
		case websocket.TextMessage:
			var msg wsClientMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "data":
				_, _ = session.Write([]byte(msg.Data))
			case "resize":
				_ = session.Resize(msg.Rows, msg.Cols)
			}
		}
	}
	<-ptyDone
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: "terminal.close", Outcome: audit.OutcomeOK,
	})
}

func parseSize(r *http.Request) (uint16, uint16) {
	rows := uint16(24)
	cols := uint16(80)
	if v := r.URL.Query().Get("rows"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			rows = uint16(n)
		}
	}
	if v := r.URL.Query().Get("cols"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			cols = uint16(n)
		}
	}
	return rows, cols
}
