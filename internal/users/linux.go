// Package users manages Linux system users via argv-exec of the standard
// shadow-utils, plus authorized_keys files via os ops. No shell strings.
package users

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type User struct {
	Name    string `json:"name"`
	UID     int    `json:"uid"`
	GID     int    `json:"gid"`
	Gecos   string `json:"gecos"`
	Home    string `json:"home"`
	Shell   string `json:"shell"`
	IsSystem bool  `json:"is_system"`
}

var nameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

func ValidName(s string) bool { return nameRe.MatchString(s) }

// List reads /etc/passwd and returns all users with UID >= 1000 by default.
// If includeSystem is true, system accounts are included too.
func List(includeSystem bool) ([]User, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []User
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		uid, _ := strconv.Atoi(parts[2])
		gid, _ := strconv.Atoi(parts[3])
		sys := uid < 1000 || uid == 65534
		if sys && !includeSystem {
			continue
		}
		out = append(out, User{
			Name: parts[0], UID: uid, GID: gid,
			Gecos: parts[4], Home: parts[5], Shell: parts[6], IsSystem: sys,
		})
	}
	return out, scanner.Err()
}

type CreateOpts struct {
	Name  string
	Gecos string
	Shell string
	Home  string // empty = default /home/<name>
	System bool
}

func Create(o CreateOpts) error {
	if !ValidName(o.Name) {
		return errors.New("invalid username")
	}
	args := []string{"-m"}
	if o.Shell != "" {
		if !validShell(o.Shell) {
			return errors.New("invalid shell")
		}
		args = append(args, "-s", o.Shell)
	}
	if o.Home != "" {
		if !validPath(o.Home) {
			return errors.New("invalid home dir")
		}
		args = append(args, "-d", o.Home)
	}
	if o.Gecos != "" {
		if strings.ContainsAny(o.Gecos, ":\n") {
			return errors.New("invalid gecos")
		}
		args = append(args, "-c", o.Gecos)
	}
	if o.System {
		args = append(args, "-r")
	}
	args = append(args, o.Name)
	return runCmd("useradd", args...)
}

func Delete(name string, removeHome bool) error {
	if !ValidName(name) {
		return errors.New("invalid username")
	}
	args := []string{}
	if removeHome {
		args = append(args, "-r")
	}
	args = append(args, name)
	return runCmd("userdel", args...)
}

// SetPassword pipes a password into `passwd --stdin <user>` via chpasswd format.
// Using chpasswd is portable across distros.
func SetPassword(name, password string) error {
	if !ValidName(name) {
		return errors.New("invalid username")
	}
	if strings.ContainsAny(password, "\n\r") || password == "" {
		return errors.New("invalid password")
	}
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(name + ":" + password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chpasswd: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func validShell(s string) bool {
	if !strings.HasPrefix(s, "/") || strings.Contains(s, "..") {
		return false
	}
	// Must exist in /etc/shells to be considered safe.
	f, err := os.Open("/etc/shells")
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == s {
			return true
		}
	}
	return false
}

func validPath(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	return true
}

// --- SSH keys ---

type SSHKey struct {
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	Raw         string `json:"raw"`
}

func authorizedKeysPath(username string) (string, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return "", err
	}
	if u.HomeDir == "" {
		return "", errors.New("user has no home directory")
	}
	return filepath.Join(u.HomeDir, ".ssh", "authorized_keys"), nil
}

func ListKeys(username string) ([]SSHKey, error) {
	if !ValidName(username) {
		return nil, errors.New("invalid username")
	}
	path, err := authorizedKeysPath(username)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SSHKey{}, nil
		}
		return nil, err
	}
	defer f.Close()
	return parseKeys(f), nil
}

func parseKeys(r io.Reader) []SSHKey {
	var out []SSHKey
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, ok := parseKeyLine(line)
		if !ok {
			continue
		}
		out = append(out, k)
	}
	return out
}

func parseKeyLine(line string) (SSHKey, bool) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return SSHKey{}, false
	}
	keyType := parts[0]
	keyBlob := parts[1]
	comment := ""
	if len(parts) == 3 {
		comment = parts[2]
	}
	raw, err := base64.StdEncoding.DecodeString(keyBlob)
	if err != nil {
		return SSHKey{}, false
	}
	sum := sha256.Sum256(raw)
	fp := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
	return SSHKey{Type: keyType, Fingerprint: fp, Comment: comment, Raw: line}, true
}

// AddKey appends a public key to authorized_keys, creating the file with safe perms.
func AddKey(username, keyLine string) (SSHKey, error) {
	keyLine = strings.TrimSpace(keyLine)
	k, ok := parseKeyLine(keyLine)
	if !ok {
		return SSHKey{}, errors.New("could not parse SSH key")
	}
	if !ValidName(username) {
		return SSHKey{}, errors.New("invalid username")
	}
	u, err := user.Lookup(username)
	if err != nil {
		return SSHKey{}, err
	}
	path, err := authorizedKeysPath(username)
	if err != nil {
		return SSHKey{}, err
	}
	sshDir := filepath.Dir(path)
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return SSHKey{}, err
	}
	existing, _ := ListKeys(username)
	for _, e := range existing {
		if e.Fingerprint == k.Fingerprint {
			return e, nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return SSHKey{}, err
	}
	defer f.Close()
	if _, err := f.WriteString(keyLine + "\n"); err != nil {
		return SSHKey{}, err
	}
	// Chown to owner.
	if uid, err := strconv.Atoi(u.Uid); err == nil {
		if gid, err := strconv.Atoi(u.Gid); err == nil {
			_ = os.Chown(path, uid, gid)
			_ = os.Chown(sshDir, uid, gid)
		}
	}
	return k, nil
}

// DeleteKey removes the key with the given fingerprint (URL-safe-encoded).
func DeleteKey(username, fpEnc string) error {
	if !ValidName(username) {
		return errors.New("invalid username")
	}
	fp, err := decodeFingerprint(fpEnc)
	if err != nil {
		return err
	}
	path, err := authorizedKeysPath(username)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var kept []string
	removed := false
	for _, line := range strings.Split(string(data), "\n") {
		k, ok := parseKeyLine(strings.TrimSpace(line))
		if ok && k.Fingerprint == fp {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return errors.New("key not found")
	}
	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o600)
}

func decodeFingerprint(s string) (string, error) {
	// Accept either "SHA256:..." passed directly or hex-encoded for URL safety.
	if strings.HasPrefix(s, "SHA256:") {
		return s, nil
	}
	if b, err := hex.DecodeString(s); err == nil {
		return string(b), nil
	}
	return "", errors.New("invalid fingerprint format")
}
