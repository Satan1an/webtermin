// Package pty spawns a shell inside a PTY and exposes it as an io.ReadWriteCloser.
package pty

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

// validShells is the in-process cache of /etc/shells; allowlisting the shell
// path prevents a misconfigured (or tampered) config.yaml from launching an
// arbitrary binary as the panel's PTY shell.
var validShells = func() map[string]bool {
	m := map[string]bool{}
	f, err := os.Open("/etc/shells")
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			m[line] = true
		}
	}
	return m
}()

func isValidShell(path string) bool {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return false
	}
	// /etc/shells is the canonical allowlist on POSIX systems; fall back to
	// hardcoded common shells if /etc/shells couldn't be read (very minimal
	// container images sometimes lack it).
	if len(validShells) > 0 {
		return validShells[path]
	}
	switch path {
	case "/bin/sh", "/bin/bash", "/bin/dash", "/usr/bin/bash", "/usr/bin/zsh", "/bin/zsh":
		return true
	}
	return false
}

type Session struct {
	cmd *exec.Cmd
	tty *os.File
}

func (s *Session) Read(p []byte) (int, error)  { return s.tty.Read(p) }
func (s *Session) Write(p []byte) (int, error) { return s.tty.Write(p) }
func (s *Session) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.tty != nil {
		_ = s.tty.Close()
	}
	if s.cmd != nil {
		_, _ = s.cmd.Process.Wait()
	}
	return nil
}

func (s *Session) Resize(rows, cols uint16) error {
	return pty.Setsize(s.tty, &pty.Winsize{Rows: rows, Cols: cols})
}

// Start spawns a login shell as the given Linux user. If username is empty
// or "root", the shell runs as the current process user (typically root,
// since webtermin needs root for system management).
func Start(username, defaultShell string, rows, cols uint16) (*Session, error) {
	shell := defaultShell
	var uid, gid int = -1, -1
	var home string

	if username != "" {
		u, err := user.Lookup(username)
		if err != nil {
			return nil, err
		}
		if uid64, err := strconv.Atoi(u.Uid); err == nil {
			uid = uid64
		}
		if gid64, err := strconv.Atoi(u.Gid); err == nil {
			gid = gid64
		}
		home = u.HomeDir
		// We can't easily query the user's shell without parsing /etc/passwd;
		// fall back to bash if no override.
		if shell == "" {
			shell = "/bin/bash"
		}
	}
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}
	}
	if !isValidShell(shell) {
		return nil, errors.New("shell not in /etc/shells allowlist: " + shell)
	}
	if _, err := os.Stat(shell); err != nil {
		return nil, errors.New("shell not found: " + shell)
	}

	cmd := exec.Command(shell, "-l") //nosec G204 -- shell is allowlisted via /etc/shells above

	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	)
	if home != "" {
		cmd.Env = append(cmd.Env, "HOME="+home)
		cmd.Dir = home
	}
	if uid >= 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
			Setsid:     true,
		}
	}
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, err
	}
	return &Session{cmd: cmd, tty: tty}, nil
}

var _ io.ReadWriteCloser = (*Session)(nil)
