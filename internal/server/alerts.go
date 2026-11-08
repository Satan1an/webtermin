package server

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/notify"
	"github.com/Satan1an/webtermin/internal/packages"
	"github.com/Satan1an/webtermin/internal/system"
	"github.com/Satan1an/webtermin/internal/systemd"
)

// alertOnAudit turns selected audit events into Telegram alerts. It runs
// inline after the audit write, so it only does cheap work and enqueues.
func (s *Server) alertOnAudit(e audit.Event) {
	if !s.Notify.Enabled() {
		return
	}
	who := fmt.Sprintf("%s from %s", e.Username, e.IP)
	switch {
	case (e.Action == "auth.login" || e.Action == "auth.login.oidc") && e.Outcome == audit.OutcomeOK:
		// The event is already stored, so a familiar address counts at least twice.
		if n, err := s.Store.CountLogins(e.Username, e.IP); err == nil && n <= 1 {
			s.Notify.Notify(notify.EventLogin, "sign-in from a new address", who)
		}
	case e.Action == "auth.login" && e.Detail == auth.ErrLockedOut.Error():
		s.Notify.Notify(notify.EventLockout, "sign-in locked out", who, "Repeated failed passwords.")
	case e.Action == "auth.setup" && e.Outcome == audit.OutcomeOK:
		s.Notify.Notify(notify.EventSetup, "first-run admin created", who)
	case e.Action == "system.reboot":
		s.Notify.Notify(notify.EventReboot, "reboot requested", who, e.Detail)
	case e.Outcome == audit.OutcomeDenied && !strings.HasPrefix(e.Action, "auth."),
		e.Outcome == audit.OutcomeError && strings.Contains(e.Detail, "blocked by safety checks"):
		s.Notify.Notify(notify.EventDenied, "refused: "+e.Action, who, strings.TrimSpace(e.Target+" "+e.Detail))
	}
}

// monitorState remembers what was already reported so a full disk or a failed
// unit produces one alert, not one a minute.
type monitorState struct {
	diskAlerted map[string]bool
	failed      map[string]bool
	security    int
	primed      bool
}

// monitor polls host health once a minute while alerts are enabled.
func (s *Server) monitor(ctx context.Context) {
	st := &monitorState{diskAlerted: map[string]bool{}, failed: map[string]bool{}}
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		s.monitorTick(ctx, st)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Server) monitorTick(ctx context.Context, st *monitorState) {
	if m, err := system.GetMetrics(ctx, 0); err == nil {
		for _, d := range m.Disks {
			s.checkDisk(st, d.Mount, d.UsedPercent)
		}
	}
	if units, err := systemd.List(ctx, ""); err == nil {
		var failed []string
		for _, u := range units {
			if u.ActiveState == "failed" {
				failed = append(failed, u.Name)
			}
		}
		s.checkFailedUnits(st, failed)
	}
	if up := packages.GetUpdates(); !up.CheckedAt.IsZero() {
		s.checkSecurityUpdates(st, up)
	}
	st.primed = true
}

func (s *Server) checkDisk(st *monitorState, mount string, pct float64) {
	limit := float64(s.Notify.DiskPct())
	switch {
	case pct >= limit && !st.diskAlerted[mount]:
		st.diskAlerted[mount] = true
		s.Notify.Notify(notify.EventDisk, "disk almost full", fmt.Sprintf("%s is %.0f%% used (alert at %.0f%%).", mount, pct, limit))
	case pct < limit-5:
		// Hysteresis: re-arm only after usage clearly drops.
		delete(st.diskAlerted, mount)
	}
}

func (s *Server) checkFailedUnits(st *monitorState, failed []string) {
	now := map[string]bool{}
	for _, u := range failed {
		now[u] = true
		if !st.failed[u] {
			s.Notify.Notify(notify.EventServiceFailed, "unit failed: "+u, "Check: journalctl -u "+u)
		}
	}
	for u := range st.failed {
		if !now[u] {
			s.Notify.Notify(notify.EventServiceFailed, "unit recovered: "+u)
		}
	}
	st.failed = now
}

func (s *Server) checkSecurityUpdates(st *monitorState, up packages.Updates) {
	if up.Security > st.security {
		var names []string
		for _, p := range up.Packages {
			if p.Security {
				names = append(names, p.Name)
			}
		}
		slices.Sort(names)
		if len(names) > 15 {
			names = append(names[:15], "…")
		}
		s.Notify.Notify(notify.EventUpdates, fmt.Sprintf("%d security update(s) available", up.Security),
			strings.Join(names, ", "), "Review and upgrade from the dashboard.")
	}
	st.security = up.Security
}
