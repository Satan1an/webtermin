// Package pty spawns a shell inside a PTY and exposes it as an io.ReadWriteCloser.
package pty

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/creack/pty"
)

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
	if _, err := os.Stat(shell); err != nil {
		return nil, errors.New("shell not found: " + shell)
	}

	cmd := exec.Command(shell, "-l")
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
