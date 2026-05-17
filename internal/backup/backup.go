// Package backup builds and verifies tar.gz snapshots of admin-supplied paths
// from the host filesystem. The intended use is "panic button" backups of
// /etc and /var/lib/webtermin before a risky change — small enough to keep
// many of them, simple enough to restore with `tar xzf` from any shell.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Default paths included in a backup when the caller doesn't override.
// These cover system config + webtermin's own state — together that's
// enough to bring the panel back to its current state on a new box.
var DefaultPaths = []string{
	"/etc/webtermin",
	"/var/lib/webtermin",
}

// nameRe restricts user-supplied backup names so we don't get path traversal
// or shell-meta surprises in filenames.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func ValidName(s string) bool { return nameRe.MatchString(s) }

// ValidPath rejects relative paths, traversal, and globs. We don't allow the
// root filesystem (`/`) — it would happily try to tar `/proc`, `/sys`, etc.
func ValidPath(p string) bool {
	if !filepath.IsAbs(p) || p == "/" {
		return false
	}
	if strings.Contains(p, "..") || strings.ContainsAny(p, "*?[") {
		return false
	}
	return true
}

// Create writes paths to a .tar.gz under outDir/<name>-<ts>.tar.gz and returns
// (final path, size in bytes). Failures clean up any partial file.
func Create(outDir, name string, paths []string) (string, int64, error) {
	if !ValidName(name) {
		return "", 0, fmt.Errorf("invalid backup name")
	}
	if len(paths) == 0 {
		return "", 0, fmt.Errorf("no paths to back up")
	}
	for _, p := range paths {
		if !ValidPath(p) {
			return "", 0, fmt.Errorf("invalid path: %s", p)
		}
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", 0, err
	}
	ts := time.Now().UTC().Format("20060102-150405")
	fname := name + "-" + ts + ".tar.gz"
	full := filepath.Join(outDir, fname)
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	for _, p := range paths {
		if err := tarTree(tw, p); err != nil {
			tw.Close()
			gz.Close()
			os.Remove(full)
			return "", 0, fmt.Errorf("tar %s: %w", p, err)
		}
	}
	if err := tw.Close(); err != nil {
		gz.Close()
		os.Remove(full)
		return "", 0, err
	}
	if err := gz.Close(); err != nil {
		os.Remove(full)
		return "", 0, err
	}

	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	return full, fi.Size(), nil
}

// tarTree walks `root` and writes every regular file, directory, and symlink
// it encounters into tw with paths relative to `/`. Sockets, devices, fifos
// are skipped — they're meaningless on restore anyway.
func tarTree(tw *tar.Writer, root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			// Tolerate missing roots — caller may have over-listed and the
			// directory simply doesn't exist on this host yet.
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return addEntry(tw, root, info)
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return addEntry(tw, path, info)
	})
}

func addEntry(tw *tar.Writer, path string, info os.FileInfo) error {
	mode := info.Mode()
	if mode&(os.ModeSocket|os.ModeDevice|os.ModeNamedPipe|os.ModeCharDevice) != 0 {
		return nil
	}
	link := ""
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		link = target
	}
	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	// Strip the leading slash so `tar xzf` doesn't try to write to absolute
	// paths by default; restorers can still `tar xzf -C /` to put it back
	// where it came from.
	hdr.Name = strings.TrimPrefix(path, "/")
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if mode.IsRegular() {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
	}
	return nil
}
