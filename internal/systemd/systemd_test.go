package systemd

import (
	"strings"
	"testing"
)

func TestValidUnitName(t *testing.T) {
	good := []string{
		"sshd.service",
		"nginx.service",
		"systemd-resolved.service",
		"webtermin.service",
		"getty@tty1.service",                  // template instance
		"dbus.socket",
		"daily-backup.timer",
		"multi-user.target",
		"systemd-tmpfiles-setup.path",
		"home-user-.mount",
		"docker.service",
		"my_service.service",
	}
	for _, n := range good {
		if !ValidUnitName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}

	bad := []string{
		"",                              // empty
		"sshd",                          // no suffix
		"sshd.unknown",                  // bad suffix
		"sshd.service ; rm -rf /",       // shell metachars
		"foo bar.service",               // space
		"/etc/passwd",                   // path
		"../sshd.service",               // traversal
		"foo`reboot`.service",           // backticks
		"foo$(whoami).service",          // command substitution
		"sshd.service\nnginx.service",   // newline
		strings.Repeat("a", 201) + ".service", // oversize
	}
	for _, n := range bad {
		if ValidUnitName(n) {
			t.Errorf("expected %q to be REJECTED, but it passed", n)
		}
	}
}

func TestValidAction(t *testing.T) {
	for _, a := range []string{"start", "stop", "restart", "reload", "enable", "disable"} {
		if !ValidAction(a) {
			t.Errorf("expected %q to be valid action", a)
		}
	}
	for _, a := range []string{
		"", "kill", "mask", "unmask", "edit",
		"start; rm",
		"START", // case-sensitive intentionally
	} {
		if ValidAction(a) {
			t.Errorf("expected %q to be rejected as action", a)
		}
	}
}
