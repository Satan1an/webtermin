// Package cron manages per-user crontabs through the standard `crontab`
// shadow utility. Listing is `crontab -u <user> -l`; writes happen by piping
// the entire new file via stdin (`crontab -u <user> -`).
//
// Schedule strings are validated against either the classic 5-field syntax
// or the documented `@reboot`/`@daily`/etc aliases — no shell metacharacters
// are interpolated into the crontab.
package cron

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Entry is one logical crontab line.
type Entry struct {
	Line     int    `json:"line"`     // 1-based position in the file (used as id for delete)
	Schedule string `json:"schedule"` // "0 3 * * *" or "@daily"
	Command  string `json:"command"`
	Comment  string `json:"comment,omitempty"` // trailing `# ...` comment, if any
}

var (
	// One field accepts digits, *, /, -, , (range/step/list), with optional name
	// for month/dow (jan-dec, mon-sun) — but we keep it conservative: numeric or
	// the lone star/step forms. This catches well-formed schedules and rejects
	// anything weird.
	fieldRe = regexp.MustCompile(`^[\d*/,-]+$`)

	allowedAliases = map[string]bool{
		"@reboot":   true,
		"@yearly":   true,
		"@annually": true,
		"@monthly":  true,
		"@weekly":   true,
		"@daily":    true,
		"@midnight": true,
		"@hourly":   true,
	}
)

// ValidSchedule reports whether s is a syntactically acceptable cron schedule.
func ValidSchedule(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "@") {
		return allowedAliases[s]
	}
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return false
	}
	for _, f := range fields {
		if !fieldRe.MatchString(f) {
			return false
		}
	}
	return true
}

// ValidCommand rejects characters that would break the crontab file syntax.
// Newlines especially — they'd inject a whole new entry.
func ValidCommand(s string) bool {
	if s == "" || len(s) > 4096 {
		return false
	}
	for _, r := range s {
		if r == '\n' || r == '\r' || r == 0 {
			return false
		}
	}
	return true
}

// List returns the user's current crontab entries. Returns an empty slice if
// the user has no crontab installed yet — that's a normal state, not an error.
func List(user string) ([]Entry, error) {
	if user == "" {
		return nil, errors.New("user is required")
	}
	cmd := exec.Command("crontab", "-u", user, "-l")
	out, err := cmd.Output()
	if err != nil {
		// `crontab -l` exits non-zero when the user has no crontab. Detect that
		// by stderr contents and treat as "empty list".
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "no crontab") {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("crontab -l: %w", err)
	}
	return parseCrontab(string(out)), nil
}

func parseCrontab(text string) []Entry {
	var out []Entry
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		schedule, command, comment, ok := splitEntry(raw)
		if !ok {
			continue
		}
		out = append(out, Entry{
			Line: lineNo, Schedule: schedule, Command: command, Comment: comment,
		})
	}
	return out
}

// splitEntry splits a single non-comment crontab line into (schedule, command,
// trailing-comment). Returns ok=false for malformed lines.
func splitEntry(line string) (schedule, command, comment string, ok bool) {
	// Trailing comment on the same line is rare but legal in vixie-cron.
	if i := strings.Index(line, " # "); i >= 0 {
		comment = strings.TrimSpace(line[i+3:])
		line = line[:i]
	}
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "@") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !allowedAliases[fields[0]] {
			return "", "", "", false
		}
		return fields[0], strings.TrimSpace(strings.TrimPrefix(line, fields[0])), comment, true
	}

	// Pull the first 5 whitespace-separated tokens as schedule fields, rest as command.
	parts := strings.Fields(line)
	if len(parts) < 6 {
		return "", "", "", false
	}
	for _, f := range parts[:5] {
		if !fieldRe.MatchString(f) {
			return "", "", "", false
		}
	}
	schedule = strings.Join(parts[:5], " ")
	command = strings.Join(parts[5:], " ")
	return schedule, command, comment, true
}

// Add appends a new entry to the user's crontab.
func Add(user string, e Entry) error {
	if !ValidSchedule(e.Schedule) {
		return errors.New("invalid schedule")
	}
	if !ValidCommand(e.Command) {
		return errors.New("invalid command")
	}
	if e.Comment != "" {
		for _, r := range e.Comment {
			if r == '\n' || r == '\r' || r == 0 {
				return errors.New("invalid comment")
			}
		}
	}
	existing, err := raw(user)
	if err != nil {
		return err
	}
	line := e.Schedule + " " + e.Command
	if e.Comment != "" {
		line += " # " + e.Comment
	}
	next := existing
	if !strings.HasSuffix(next, "\n") && next != "" {
		next += "\n"
	}
	next += line + "\n"
	return write(user, next)
}

// DeleteLine removes the entry that originally sat on line `lineNo` (1-based,
// matching what List returned). No-op if the line number is out of range.
func DeleteLine(user string, lineNo int) error {
	if lineNo <= 0 {
		return errors.New("invalid line number")
	}
	existing, err := raw(user)
	if err != nil {
		return err
	}
	lines := strings.Split(existing, "\n")
	if lineNo > len(lines) {
		return errors.New("line not found")
	}
	// Don't touch comments; the same line index that List returned is the file's
	// line index — so just remove that line.
	idx := lineNo - 1
	lines = append(lines[:idx], lines[idx+1:]...)
	return write(user, strings.Join(lines, "\n"))
}

// raw reads the crontab as a single string. Empty-on-no-crontab.
func raw(user string) (string, error) {
	cmd := exec.Command("crontab", "-u", user, "-l")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "no crontab") {
			return "", nil
		}
		return "", err
	}
	return string(out), nil
}

// write replaces the user's crontab with the supplied text.
func write(user, text string) error {
	cmd := exec.Command("crontab", "-u", user, "-")
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
