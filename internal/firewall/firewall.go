// Package firewall wraps `ufw` — the Uncomplicated Firewall — with a strict
// allowlist of rule specs. Anything that doesn't match the regex is rejected
// before we go anywhere near exec.
//
// `ufw` is the standard frontend on Debian/Ubuntu/OrangePi/RPi. On systems
// without ufw, every call returns ErrNotAvailable so the UI can surface a
// helpful message instead of a 500.
package firewall

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ErrNotAvailable = errors.New("ufw not installed on this host")

// Status mirrors what we expose to the frontend.
type Status struct {
	Available    bool   `json:"available"`
	Active       bool   `json:"active"`
	DefaultIn    string `json:"default_in"`
	DefaultOut   string `json:"default_out"`
	DefaultFwd   string `json:"default_fwd"`
	Logging      string `json:"logging"`
	Rules        []Rule `json:"rules"`
}

type Rule struct {
	Number int    `json:"number"`
	To     string `json:"to"`
	Action string `json:"action"` // "ALLOW IN", "DENY IN", "ALLOW OUT" …
	From   string `json:"from"`
}

func available() bool {
	_, err := exec.LookPath("ufw")
	return err == nil
}

// GetStatus returns a parsed view of `ufw status verbose` plus
// `ufw status numbered` for the rule list.
func GetStatus() (*Status, error) {
	if !available() {
		return &Status{Available: false}, nil
	}
	st := &Status{Available: true}

	verbose, err := exec.Command("ufw", "status", "verbose").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ufw status verbose: %v: %s", err, strings.TrimSpace(string(verbose)))
	}
	parseStatusHeader(string(verbose), st)

	numbered, err := exec.Command("ufw", "status", "numbered").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ufw status numbered: %v: %s", err, strings.TrimSpace(string(numbered)))
	}
	st.Rules = parseRules(string(numbered))
	return st, nil
}

var (
	statusLineRe  = regexp.MustCompile(`^Status:\s+(\S+)`)
	defaultsLineRe = regexp.MustCompile(`^Default:\s+(.*)$`)
	loggingLineRe = regexp.MustCompile(`^Logging:\s+(\S+)`)
	// "[ 1] 22/tcp                     ALLOW IN    Anywhere"
	ruleLineRe = regexp.MustCompile(`^\[\s*(\d+)\]\s+(.+?)\s{2,}(ALLOW|DENY|REJECT|LIMIT)\s+(IN|OUT)\s+(.+)$`)
)

func parseStatusHeader(out string, st *Status) {
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := statusLineRe.FindStringSubmatch(line); m != nil {
			st.Active = strings.EqualFold(m[1], "active")
			continue
		}
		if m := defaultsLineRe.FindStringSubmatch(line); m != nil {
			fields := strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' })
			for _, f := range fields {
				f = strings.TrimSpace(f)
				// "deny (incoming)", "allow (outgoing)", "disabled (routed)"
				parts := strings.Fields(f)
				if len(parts) < 2 {
					continue
				}
				val, kind := parts[0], strings.Trim(parts[1], "()")
				switch kind {
				case "incoming":
					st.DefaultIn = val
				case "outgoing":
					st.DefaultOut = val
				case "routed":
					st.DefaultFwd = val
				}
			}
			continue
		}
		if m := loggingLineRe.FindStringSubmatch(line); m != nil {
			st.Logging = m[1]
		}
	}
}

func parseRules(out string) []Rule {
	var rules []Rule
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		m := ruleLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		rules = append(rules, Rule{
			Number: n,
			To:     strings.TrimSpace(m[2]),
			Action: m[3] + " " + m[4],
			From:   strings.TrimSpace(m[5]),
		})
	}
	return rules
}

// ValidSpec restricts what callers can pass to `ufw allow/deny`. Accepts:
//   - bare port numbers: "22"
//   - port/proto pairs: "443/tcp", "53/udp"
//   - port ranges:      "8000:8010/tcp"
//   - named services:   "ssh", "http", "https" (must be all-lowercase a–z)
//   - cidr-from rules:  "from 10.0.0.0/8"
//                       "from 10.0.0.0/8 to any port 22 proto tcp"
//
// Anything outside this is rejected — no command injection surface, no need
// to escape, because we don't accept shell metacharacters at all.
var specPortRe = regexp.MustCompile(`^(\d{1,5})(?:[:-]\d{1,5})?(?:/(tcp|udp))?$`)
var specServiceRe = regexp.MustCompile(`^[a-z]{2,32}$`)
var specFromRe = regexp.MustCompile(
	`^from\s+[0-9a-fA-F:.]+(?:/\d{1,3})?(?:\s+to\s+any(?:\s+port\s+\d{1,5}(?:[:-]\d{1,5})?(?:\s+proto\s+(?:tcp|udp))?)?)?$`)

func ValidSpec(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 200 {
		return false
	}
	if specPortRe.MatchString(s) {
		return true
	}
	if specServiceRe.MatchString(s) {
		return true
	}
	if specFromRe.MatchString(s) {
		return true
	}
	return false
}

// Add inserts an allow/deny rule. action must be "allow" or "deny".
func Add(action, spec string) error {
	if !available() {
		return ErrNotAvailable
	}
	if action != "allow" && action != "deny" {
		return errors.New("action must be allow or deny")
	}
	if !ValidSpec(spec) {
		return errors.New("spec rejected by allowlist")
	}
	// ValidSpec guarantees no shell metacharacters; pass as separate argv tokens.
	args := append([]string{action}, strings.Fields(spec)...)
	return runUFW(args...)
}

// Delete removes the rule with the given numeric ID.
func Delete(number int) error {
	if !available() {
		return ErrNotAvailable
	}
	if number <= 0 {
		return errors.New("invalid rule number")
	}
	// `ufw --force delete N` skips the interactive confirmation.
	return runUFW("--force", "delete", strconv.Itoa(number))
}

// SetEnabled brings ufw up or takes it down. The --force flag is required for
// `enable` because the default prompt warns about possibly cutting SSH.
func SetEnabled(on bool) error {
	if !available() {
		return ErrNotAvailable
	}
	if on {
		return runUFW("--force", "enable")
	}
	return runUFW("disable")
}

func runUFW(args ...string) error {
	cmd := exec.Command("ufw", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw %s: %v: %s",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
