package docker

import (
	"strings"
	"testing"
)

func TestValidImageRef(t *testing.T) {
	good := []string{
		"nginx",
		"nginx:1.27",
		"nginx:latest",
		"library/nginx",
		"ghcr.io/satan1an/webtermin:v0.4.0",
		"localhost:5000/internal:dev",
		"alpine@sha256:abc123def456",
		"my-app.v2",
	}
	for _, r := range good {
		if !ValidImageRef(r) {
			t.Errorf("expected %q to be valid", r)
		}
	}
	bad := []string{
		"",
		"nginx ; rm -rf /",
		"$(whoami)",
		"../../etc/passwd",
		"Nginx", // upper-case rejected (engine convention is lower)
		"a b",
		"nginx\n",
		strings.Repeat("a", 300),
	}
	for _, r := range bad {
		if ValidImageRef(r) {
			t.Errorf("expected %q to be REJECTED", r)
		}
	}
}

func TestValidContainerName(t *testing.T) {
	good := []string{"", "nginx", "my-app", "app_1", "App.2", "a" + strings.Repeat("b", 127)}
	for _, n := range good {
		if !ValidContainerName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}
	bad := []string{"1", "x", "-leading-dash", "_leading-underscore", "name with space", "a;b", "../../tmp", strings.Repeat("a", 129)}
	for _, n := range bad {
		if ValidContainerName(n) {
			t.Errorf("expected %q to be REJECTED", n)
		}
	}
}

func TestValidNetworkName(t *testing.T) {
	good := []string{"web", "my-net", "internal_db.v2"}
	for _, n := range good {
		if !ValidNetworkName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}
	bad := []string{"", "-leading", "with space", "name;rm", strings.Repeat("a", 65)}
	for _, n := range bad {
		if ValidNetworkName(n) {
			t.Errorf("expected %q to be REJECTED", n)
		}
	}
}

func TestValidVolumeName(t *testing.T) {
	good := []string{"data", "my-vol", "v_2.0"}
	for _, n := range good {
		if !ValidVolumeName(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}
	bad := []string{"", "a", "-leading", "with space", "v;rm", strings.Repeat("a", 256)}
	for _, n := range bad {
		if ValidVolumeName(n) {
			t.Errorf("expected %q to be REJECTED", n)
		}
	}
}

func TestValidRestartPolicy(t *testing.T) {
	for _, p := range []string{"", "no", "always", "unless-stopped", "on-failure"} {
		if !ValidRestartPolicy(p) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	for _, p := range []string{"yes", "ALWAYS", "on-fail", "restart; rm"} {
		if ValidRestartPolicy(p) {
			t.Errorf("expected %q to be REJECTED", p)
		}
	}
}

func TestSplitImageRef(t *testing.T) {
	cases := []struct {
		in, image, tag string
	}{
		{"nginx", "nginx", ""},
		{"nginx:1.27", "nginx", "1.27"},
		{"library/nginx:latest", "library/nginx", "latest"},
		{"ghcr.io/owner/repo:v1", "ghcr.io/owner/repo", "v1"},
		// registry:port without an explicit tag — the colon belongs to the host
		{"localhost:5000/foo", "localhost:5000/foo", ""},
		// digest reference is opaque
		{"alpine@sha256:abc", "alpine@sha256:abc", ""},
	}
	for _, c := range cases {
		image, tag := splitImageRef(c.in)
		if image != c.image || tag != c.tag {
			t.Errorf("splitImageRef(%q) = (%q, %q), want (%q, %q)",
				c.in, image, tag, c.image, c.tag)
		}
	}
}

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
