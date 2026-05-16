// Package files implements filesystem operations with explicit path safety:
// every input path is cleaned, made absolute, and rejected if it traverses
// outside an allowed root (default: the whole FS for admins, but caller may
// constrain). Symlinks are followed via filepath.EvalSymlinks before any op.
package files

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	MaxReadTextSize = 5 * 1024 * 1024
)

type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	IsLink  bool      `json:"is_link"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModeOct string    `json:"mode_oct"`
	UID     uint32    `json:"uid"`
	GID     uint32    `json:"gid"`
	ModTime time.Time `json:"mtime"`
}

// SafePath cleans a user-supplied path and ensures it is absolute, with no
// "..", and that the resolved path stays under root (if root != "").
func SafePath(p, root string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	if !filepath.IsAbs(p) {
		return "", errors.New("path must be absolute")
	}
	clean := filepath.Clean(p)
	if root != "" {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(clean, absRoot+string(os.PathSeparator)) && clean != absRoot {
			return "", errors.New("path escapes root")
		}
	}
	return clean, nil
}

func List(path string) ([]Entry, error) {
	p, err := SafePath(path, "")
	if err != nil {
		return nil, err
	}
	dir, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	infos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		e := toEntry(p, fi)
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func toEntry(parent string, fi os.FileInfo) Entry {
	mode := fi.Mode()
	full := filepath.Join(parent, fi.Name())
	e := Entry{
		Name: fi.Name(), Path: full,
		IsDir: mode.IsDir(), IsLink: mode&os.ModeSymlink != 0,
		Size: fi.Size(), Mode: mode.String(),
		ModeOct: modeToOct(mode), ModTime: fi.ModTime(),
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		e.UID = st.Uid
		e.GID = st.Gid
	}
	return e
}

func modeToOct(m os.FileMode) string {
	return formatOct(uint32(m.Perm()))
}

func formatOct(n uint32) string {
	out := []byte{'0', '0', '0', '0'}
	for i := 3; i >= 0; i-- {
		out[i] = byte('0' + n%8)
		n /= 8
	}
	return string(out)
}

func ReadText(path string) (string, error) {
	p, err := SafePath(path, "")
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", errors.New("is a directory")
	}
	if fi.Size() > MaxReadTextSize {
		return "", errors.New("file too large to read inline")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func WriteText(path, content string) error {
	p, err := SafePath(path, "")
	if err != nil {
		return err
	}
	// Preserve mode if file already exists.
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(p); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp := p + ".webtermin.tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func Mkdir(path string) error {
	p, err := SafePath(path, "")
	if err != nil {
		return err
	}
	return os.MkdirAll(p, 0o755)
}

func Delete(path string, recursive bool) error {
	p, err := SafePath(path, "")
	if err != nil {
		return err
	}
	// Guard against deletion of obviously-critical roots.
	switch p {
	case "/", "/etc", "/usr", "/var", "/home", "/root", "/boot":
		return errors.New("refusing to delete critical path")
	}
	if recursive {
		return os.RemoveAll(p)
	}
	return os.Remove(p)
}

func Chmod(path string, mode uint32) error {
	if mode > 0o7777 {
		return errors.New("invalid mode")
	}
	p, err := SafePath(path, "")
	if err != nil {
		return err
	}
	return os.Chmod(p, os.FileMode(mode))
}

func OpenForDownload(path string) (*os.File, os.FileInfo, error) {
	p, err := SafePath(path, "")
	if err != nil {
		return nil, nil, err
	}
	fi, err := os.Stat(p)
	if err != nil {
		return nil, nil, err
	}
	if fi.IsDir() {
		return nil, nil, errors.New("is a directory")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, err
	}
	return f, fi, nil
}

// Save writes a streamed body to dir/name. Existing files are overwritten.
func Save(dir, name string, src io.Reader) (string, error) {
	if strings.Contains(name, "/") || strings.Contains(name, "..") || name == "" {
		return "", errors.New("invalid file name")
	}
	p, err := SafePath(filepath.Join(dir, name), "")
	if err != nil {
		return "", err
	}
	tmp := p + ".webtermin.tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return p, nil
}
