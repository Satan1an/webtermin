package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafePath_AbsoluteAndClean(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/etc/passwd", "/etc/passwd"},
		{"/etc/./passwd", "/etc/passwd"},
		{"/var//log///nginx/access.log", "/var/log/nginx/access.log"},
		{"/home/user/", "/home/user"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := SafePath(c.in, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestSafePath_RejectsRelative(t *testing.T) {
	rels := []string{
		"etc/passwd",
		"./passwd",
		"../etc/passwd",
		"",
	}
	for _, p := range rels {
		t.Run(p, func(t *testing.T) {
			if _, err := SafePath(p, ""); err == nil {
				t.Fatalf("expected error for %q, got nil", p)
			}
		})
	}
}

func TestSafePath_EnforcesRoot(t *testing.T) {
	// Anything outside the supplied root is rejected; anything inside passes.
	root := "/srv/data"
	ok := []string{
		"/srv/data",
		"/srv/data/file.txt",
		"/srv/data/sub/dir/file.txt",
	}
	for _, p := range ok {
		if _, err := SafePath(p, root); err != nil {
			t.Errorf("expected %q to be allowed under root %q, got %v", p, root, err)
		}
	}
	bad := []string{
		"/srv/data2", // sibling that starts with same prefix
		"/etc/passwd",
		"/srv",                // parent
		"/srv/data/../passwd", // gets cleaned to /srv/passwd
	}
	for _, p := range bad {
		if _, err := SafePath(p, root); err == nil {
			t.Errorf("expected %q to be REJECTED under root %q, but it was allowed", p, root)
		}
	}
}

func TestSafePath_TraversalAttempts(t *testing.T) {
	// All of these clean to something outside any reasonable root.
	bad := []string{
		"/srv/data/../../etc/passwd",
		"/srv/data/../../../../../etc/shadow",
	}
	for _, p := range bad {
		if _, err := SafePath(p, "/srv/data"); err == nil {
			t.Errorf("traversal %q was accepted under /srv/data", p)
		}
	}
}

func TestReadText_DirectoryError(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadText(dir); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected 'is a directory' error, got %v", err)
	}
}

func TestReadText_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	want := "hello, world\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("read got %q, want %q", got, want)
	}
}

func TestWriteText_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteText(path, "b"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode changed: got %o, want 0o600", fi.Mode().Perm())
	}
}

func TestDelete_RefusesCriticalRoots(t *testing.T) {
	for _, p := range []string{"/", "/etc", "/usr", "/var", "/home", "/root", "/boot"} {
		if err := Delete(p, true); err == nil {
			t.Fatalf("Delete(%q, true) should have refused", p)
		}
	}
}

func TestSave_RejectsBadFilename(t *testing.T) {
	dir := t.TempDir()
	bad := []string{"", "../escape", "evil/slash", "../../etc/passwd"}
	for _, name := range bad {
		if _, err := Save(dir, name, strings.NewReader("x")); err == nil {
			t.Fatalf("Save accepted bad filename %q", name)
		}
	}
}
