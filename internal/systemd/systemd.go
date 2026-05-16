// Package systemd wraps a small, allowlisted subset of systemd D-Bus calls.
//
// All inputs are constrained to a strict unit-name regex; no shell strings are
// constructed from user input.
package systemd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	dbus "github.com/coreos/go-systemd/v22/dbus"
)

// Allowed unit names: alnum, _, -, ., @, :, \\, .service|.socket|.timer|.target|.path|.mount
var unitRe = regexp.MustCompile(`^[A-Za-z0-9@_.\-:\\]{1,200}\.(service|socket|timer|target|path|mount|swap|slice|scope)$`)

func ValidUnitName(name string) bool { return unitRe.MatchString(name) }

type Unit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	Followed    string `json:"followed,omitempty"`
}

// List returns all known units. typeFilter, if non-empty, must be a valid suffix
// (e.g. "service") and filters by it.
func List(ctx context.Context, typeFilter string) ([]Unit, error) {
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("dbus connect: %w", err)
	}
	defer conn.Close()
	statuses, err := conn.ListUnitsContext(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Unit, 0, len(statuses))
	suffix := ""
	if typeFilter != "" {
		suffix = "." + typeFilter
	}
	for _, u := range statuses {
		if suffix != "" && !strings.HasSuffix(u.Name, suffix) {
			continue
		}
		out = append(out, Unit{
			Name: u.Name, Description: u.Description,
			LoadState: u.LoadState, ActiveState: u.ActiveState, SubState: u.SubState,
			Followed: u.Followed,
		})
	}
	return out, nil
}

type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionReload  Action = "reload"
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
)

func ValidAction(a string) bool {
	switch Action(a) {
	case ActionStart, ActionStop, ActionRestart, ActionReload, ActionEnable, ActionDisable:
		return true
	}
	return false
}

// Do performs the action against the given unit. Returns the job result for
// start/stop/restart/reload ("done", "failed", etc.), or empty for enable/disable.
func Do(ctx context.Context, unit string, action Action) (string, error) {
	if !ValidUnitName(unit) {
		return "", errors.New("invalid unit name")
	}
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	switch action {
	case ActionEnable:
		_, _, err := conn.EnableUnitFilesContext(ctx, []string{unit}, false, true)
		return "", err
	case ActionDisable:
		_, err := conn.DisableUnitFilesContext(ctx, []string{unit}, false)
		return "", err
	}
	ch := make(chan string, 1)
	var jobErr error
	switch action {
	case ActionStart:
		_, jobErr = conn.StartUnitContext(ctx, unit, "replace", ch)
	case ActionStop:
		_, jobErr = conn.StopUnitContext(ctx, unit, "replace", ch)
	case ActionRestart:
		_, jobErr = conn.RestartUnitContext(ctx, unit, "replace", ch)
	case ActionReload:
		_, jobErr = conn.ReloadUnitContext(ctx, unit, "replace", ch)
	default:
		return "", fmt.Errorf("unsupported action %s", action)
	}
	if jobErr != nil {
		return "", jobErr
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-ch:
		return result, nil
	case <-time.After(30 * time.Second):
		return "", errors.New("action timed out")
	}
}
