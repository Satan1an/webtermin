package docker

import (
	"strings"
	"testing"
)

func TestValidContainerID(t *testing.T) {
	good := []string{
		"0123456789ab",                     // exactly 12
		"0123456789abcdef0123456789abcdef", // 32
		strings.Repeat("a", 64),
	}
	for _, id := range good {
		if !ValidContainerID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}
	bad := []string{
		"",
		"0123456789",            // 10 chars — too short
		"0123456789AB",          // uppercase hex
		"0123456789ab!",         // metachar
		strings.Repeat("a", 65), // too long
		"nginx",
		"0123 4567 89ab",
		"../../etc/passwd",
	}
	for _, id := range bad {
		if ValidContainerID(id) {
			t.Errorf("expected %q to be REJECTED", id)
		}
	}
}

func TestValidAction(t *testing.T) {
	for _, a := range []string{"start", "stop", "restart", "pause", "unpause", "kill"} {
		if !ValidAction(a) {
			t.Errorf("expected %q to be valid", a)
		}
	}
	for _, a := range []string{"", "remove", "rm", "exec", "Start", "kill; rm"} {
		if ValidAction(a) {
			t.Errorf("expected %q to be REJECTED", a)
		}
	}
}
