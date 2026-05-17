package cron

import (
	"strings"
	"testing"
)

func TestValidSchedule(t *testing.T) {
	good := []string{
		"* * * * *",
		"0 3 * * *",
		"*/5 * * * *",
		"0,15,30,45 * * * *",
		"0 0 1-7 * *",
		"@reboot", "@daily", "@hourly", "@weekly", "@monthly", "@yearly", "@annually", "@midnight",
	}
	for _, s := range good {
		if !ValidSchedule(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	bad := []string{
		"",
		" ",
		"* * * *",        // only 4 fields
		"* * * * * *",    // 6 fields
		"@every-minute",  // unknown alias
		"0 3 * * * rm -rf /", // schedule plus command in one
		"0 3 * * mon",    // named DOW — we don't accept names
		"$()@daily",
		"@reboot; rm",
	}
	for _, s := range bad {
		if ValidSchedule(s) {
			t.Errorf("expected %q to be REJECTED", s)
		}
	}
}

func TestValidCommand(t *testing.T) {
	if !ValidCommand("/usr/bin/echo hello") {
		t.Error("normal command rejected")
	}
	if ValidCommand("") {
		t.Error("empty command accepted")
	}
	if ValidCommand("foo\nbar") {
		t.Error("newline-injected command accepted")
	}
	if ValidCommand("foo\x00bar") {
		t.Error("NUL byte accepted")
	}
	if ValidCommand(strings.Repeat("a", 5000)) {
		t.Error("oversize command accepted")
	}
}

func TestParseCrontab(t *testing.T) {
	src := `# comment header
SHELL=/bin/bash

0 3 * * * /usr/local/bin/backup.sh # nightly backup
*/15 * * * * /opt/check.sh
@reboot /opt/start-once.sh
# another comment
malformed line here
0 12 * * * /usr/bin/lunch
`
	entries := parseCrontab(src)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Schedule != "0 3 * * *" || entries[0].Command != "/usr/local/bin/backup.sh" {
		t.Errorf("entry[0]: %+v", entries[0])
	}
	if entries[0].Comment != "nightly backup" {
		t.Errorf("trailing comment: %q", entries[0].Comment)
	}
	if entries[2].Schedule != "@reboot" || entries[2].Command != "/opt/start-once.sh" {
		t.Errorf("@reboot entry: %+v", entries[2])
	}
}

func TestSplitEntry_RejectsShellMetacharsInSchedule(t *testing.T) {
	if _, _, _, ok := splitEntry("$(whoami) * * * * /bin/sh"); ok {
		t.Error("schedule with shell substitution accepted")
	}
}
