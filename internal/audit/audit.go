// Package audit centralises audit-log writes so call sites stay terse.
package audit

import (
	"log/slog"

	"github.com/Satan1an/webtermin/internal/store"
)

type Logger struct {
	Store *store.Store
	Log   *slog.Logger
}

func New(s *store.Store, l *slog.Logger) *Logger { return &Logger{Store: s, Log: l} }

const (
	OutcomeOK     = "ok"
	OutcomeError  = "error"
	OutcomeDenied = "denied"
)

type Event struct {
	UserID   *int64
	Username string
	IP       string
	Action   string
	Target   string
	Outcome  string
	Detail   string
}

func (l *Logger) Write(e Event) {
	if err := l.Store.WriteAudit(store.AuditEntry{
		UserID:   e.UserID,
		Username: e.Username,
		IP:       e.IP,
		Action:   e.Action,
		Target:   e.Target,
		Outcome:  e.Outcome,
		Detail:   e.Detail,
	}); err != nil {
		l.Log.Error("audit write failed", "err", err, "action", e.Action)
	}
	l.Log.Info("audit",
		"action", e.Action,
		"target", e.Target,
		"outcome", e.Outcome,
		"user", e.Username,
		"ip", e.IP,
	)
}
