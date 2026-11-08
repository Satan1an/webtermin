package server

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Satan1an/webtermin/internal/notify"
	"github.com/Satan1an/webtermin/internal/packages"
)

// newAlertServer builds a Server with a real notifier whose queue is never
// sent (Run isn't started), so tests count alerts with notify.Drain.
func newAlertServer(t *testing.T) *Server {
	t.Helper()
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Server{Log: lg, Notify: notify.New(notify.Config{BotToken: "t", ChatID: "1", DiskPct: 90}, lg)}
}

func drain(s *Server) int { return notify.Drain(s.Notify) }

func TestCheckDisk_AlertsOnceWithHysteresis(t *testing.T) {
	s := newAlertServer(t)
	st := &monitorState{diskAlerted: map[string]bool{}, failed: map[string]bool{}}
	s.checkDisk(st, "/", 91)
	s.checkDisk(st, "/", 95)
	if n := drain(s); n != 1 {
		t.Fatalf("crossing once should alert once, got %d", n)
	}
	s.checkDisk(st, "/", 88) // still within hysteresis band
	s.checkDisk(st, "/", 92)
	if n := drain(s); n != 0 {
		t.Fatalf("dipping 2%% must not re-arm, got %d alerts", n)
	}
	s.checkDisk(st, "/", 80)
	s.checkDisk(st, "/", 93)
	if n := drain(s); n != 1 {
		t.Fatalf("after a real drop the next crossing alerts again, got %d", n)
	}
}

func TestCheckFailedUnits_FailAndRecover(t *testing.T) {
	s := newAlertServer(t)
	st := &monitorState{diskAlerted: map[string]bool{}, failed: map[string]bool{}}
	s.checkFailedUnits(st, []string{"nginx.service"})
	s.checkFailedUnits(st, []string{"nginx.service"})
	if n := drain(s); n != 1 {
		t.Fatalf("a unit stuck in failed alerts once, got %d", n)
	}
	s.checkFailedUnits(st, nil)
	if n := drain(s); n != 1 {
		t.Fatalf("recovery should alert once, got %d", n)
	}
}

func TestCheckSecurityUpdates_OnlyOnIncrease(t *testing.T) {
	s := newAlertServer(t)
	st := &monitorState{}
	up := packages.Updates{Security: 2, Packages: []packages.Update{{Name: "libc6", Security: true}, {Name: "openssl", Security: true}}}
	s.checkSecurityUpdates(st, up)
	s.checkSecurityUpdates(st, up)
	up.Security = 1
	s.checkSecurityUpdates(st, up)
	if n := drain(s); n != 1 {
		t.Fatalf("want one alert for the first 2 security updates, got %d", n)
	}
}
