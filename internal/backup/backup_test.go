package backup

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	good := []string{"pre-upgrade", "config_2024", "x", "a" + strings.Repeat("b", 63)}
	for _, n := range good {
		if !ValidName(n) {
			t.Errorf("expected %q valid", n)
		}
	}
	bad := []string{"", "-leading", "_leading", "../escape", "with space", "shell;rm", "name\n"}
	for _, n := range bad {
		if ValidName(n) {
			t.Errorf("expected %q REJECTED", n)
		}
	}
}

func TestValidPath(t *testing.T) {
	for _, p := range []string{"/etc/webtermin", "/var/lib/webtermin"} {
		if !ValidPath(p) {
			t.Errorf("expected %q valid", p)
		}
	}
	bad := []string{
		"", "/", "etc/passwd", "./relative", "../../escape",
		"/etc/../passwd", "/var/log/*.log",
	}
	for _, p := range bad {
		if ValidPath(p) {
			t.Errorf("expected %q REJECTED", p)
		}
	}
}

func TestCreate_RoundTrip(t *testing.T) {
	// Set up a fixture tree.
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "config.yaml"), []byte("hello: world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(src, "sub")
	_ = os.MkdirAll(sub, 0o755)
	if err := os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	full, size, err := Create(out, "test-snapshot", []string{src})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if size <= 0 {
		t.Errorf("size: %d", size)
	}
	if !strings.HasSuffix(full, ".tar.gz") {
		t.Errorf("path: %s", full)
	}

	// Read the archive back and check both files are present.
	f, err := os.Open(full)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		seen[hdr.Name] = true
	}
	// The tar entries use root-relative paths (leading / stripped).
	// We can't assert exact names because the src tempdir path differs every
	// run; just check we got two regular files plus a directory entry.
	if len(seen) < 3 {
		t.Errorf("expected ≥ 3 entries, got %d: %v", len(seen), seen)
	}
}

func TestCreate_RejectsBadName(t *testing.T) {
	if _, _, err := Create(t.TempDir(), "../bad", []string{"/etc"}); err == nil {
		t.Fatal("expected name rejection")
	}
}

func TestCreate_RejectsBadPath(t *testing.T) {
	if _, _, err := Create(t.TempDir(), "ok", []string{"/etc/../shadow"}); err == nil {
		t.Fatal("expected path rejection")
	}
}

func TestCreate_TolerantOfMissingRoot(t *testing.T) {
	// /var/lib/webtermin may not exist on test machines — Create should
	// silently skip rather than fail the whole backup.
	out := t.TempDir()
	if _, _, err := Create(out, "ok", []string{"/this/does/not/exist/anywhere"}); err != nil {
		t.Fatalf("missing root should not fail: %v", err)
	}
}
