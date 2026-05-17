// Package packages wraps the system package manager (apt or dnf, auto-detected
// at process start). Every shell-out uses argv slices and the package name
// itself is validated against a strict regex before exec.
package packages

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Manager is one of "apt" or "dnf". Detected lazily by Detect().
type Manager string

const (
	APT     Manager = "apt"
	DNF     Manager = "dnf"
	Unknown Manager = ""
)

// Detect picks the first manager whose binary is on PATH. apt wins over dnf
// on hybrid systems because that's the dominant choice on the panel's
// primary target (Debian/Ubuntu/Armbian).
func Detect() Manager {
	if _, err := exec.LookPath("apt-get"); err == nil {
		return APT
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return DNF
	}
	return Unknown
}

// Package summary entry from the local index.
type Package struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

// nameRe blocks any character that could break out of argv on a poorly
// written shell. Real package names use lower-case alnum + `.+-_~:`.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.+~_:-]{0,127}$`)

func ValidName(s string) bool { return nameRe.MatchString(s) }

// Search lists candidates whose name or short description contains `query`.
// We cap output to 100 rows — the UI displays them in a virtual list anyway,
// and `apt search` on a large index is slow if we ask for more.
func Search(ctx context.Context, query string) ([]Package, error) {
	if query == "" {
		return nil, errors.New("empty query")
	}
	if !searchQueryRe.MatchString(query) {
		return nil, errors.New("invalid query (alnum + .-_:)")
	}
	switch Detect() {
	case APT:
		return aptSearch(ctx, query)
	case DNF:
		return dnfSearch(ctx, query)
	}
	return nil, errors.New("no supported package manager found")
}

var searchQueryRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.+~_:-]{0,127}$`)

// ListInstalled is the local equivalent of `apt list --installed`.
func ListInstalled(ctx context.Context) ([]Package, error) {
	switch Detect() {
	case APT:
		return aptListInstalled(ctx)
	case DNF:
		return dnfListInstalled(ctx)
	}
	return nil, errors.New("no supported package manager found")
}

// Install performs a non-interactive install of one or more named packages.
// Names are revalidated here even though the HTTP layer does the same — this
// is the last line before exec.
func Install(ctx context.Context, names ...string) error {
	return run(ctx, "install", names...)
}

func Remove(ctx context.Context, names ...string) error {
	return run(ctx, "remove", names...)
}

func Upgrade(ctx context.Context, names ...string) error {
	if len(names) == 0 {
		return run(ctx, "upgrade-all")
	}
	return run(ctx, "upgrade", names...)
}

func run(ctx context.Context, op string, names ...string) error {
	for _, n := range names {
		if !ValidName(n) {
			return fmt.Errorf("invalid package name %q", n)
		}
	}
	var args []string
	switch Detect() {
	case APT:
		base := []string{"-y", "-q", "-o", "Dpkg::Options::=--force-confdef"}
		switch op {
		case "install":
			args = append([]string{"install"}, append(base, names...)...)
		case "remove":
			args = append([]string{"remove"}, append(base, names...)...)
		case "upgrade":
			args = append([]string{"install", "--only-upgrade"}, append(base, names...)...)
		case "upgrade-all":
			args = append([]string{"upgrade"}, base...)
		default:
			return fmt.Errorf("unsupported op %q", op)
		}
		cmd := exec.CommandContext(ctx, "apt-get", args...)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("apt-get %s: %v: %s", op, err, strings.TrimSpace(string(out)))
		}
	case DNF:
		base := []string{"-y"}
		switch op {
		case "install":
			args = append([]string{"install"}, append(base, names...)...)
		case "remove":
			args = append([]string{"remove"}, append(base, names...)...)
		case "upgrade":
			args = append([]string{"upgrade"}, append(base, names...)...)
		case "upgrade-all":
			args = append([]string{"upgrade"}, base...)
		default:
			return fmt.Errorf("unsupported op %q", op)
		}
		cmd := exec.CommandContext(ctx, "dnf", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("dnf %s: %v: %s", op, err, strings.TrimSpace(string(out)))
		}
	default:
		return errors.New("no supported package manager found")
	}
	return nil
}

// --- apt parsers ---

// apt search output:
//
//	nginx/stable 1.27-1 arm64
//	  small, powerful, scalable web/proxy server
func aptSearch(ctx context.Context, query string) ([]Package, error) {
	cmd := exec.CommandContext(ctx, "apt-cache", "search", "--names-only", query)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("apt-cache search: %w", err)
	}
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		i := strings.Index(line, " - ")
		if i <= 0 {
			continue
		}
		pkgs = append(pkgs, Package{Name: line[:i], Description: line[i+3:]})
		if len(pkgs) >= 100 {
			break
		}
	}
	return pkgs, nil
}

func aptListInstalled(ctx context.Context) ([]Package, error) {
	cmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f", "${Package}\t${Version}\t${binary:Summary}\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dpkg-query: %w", err)
	}
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 3)
		if len(fields) < 2 {
			continue
		}
		desc := ""
		if len(fields) == 3 {
			desc = fields[2]
		}
		pkgs = append(pkgs, Package{
			Name: fields[0], Version: fields[1], Description: desc, Installed: true,
		})
	}
	return pkgs, nil
}

// --- dnf parsers ---

func dnfSearch(ctx context.Context, query string) ([]Package, error) {
	cmd := exec.CommandContext(ctx, "dnf", "search", "--quiet", query)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dnf search: %w", err)
	}
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// "name.arch : description"
		if i := strings.Index(line, " : "); i > 0 {
			name := strings.SplitN(line[:i], ".", 2)[0]
			pkgs = append(pkgs, Package{Name: name, Description: line[i+3:]})
		}
		if len(pkgs) >= 100 {
			break
		}
	}
	return pkgs, nil
}

func dnfListInstalled(ctx context.Context) ([]Package, error) {
	cmd := exec.CommandContext(ctx, "dnf", "list", "installed", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dnf list: %w", err)
	}
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for i := 0; scanner.Scan(); i++ {
		if i == 0 && strings.HasPrefix(scanner.Text(), "Installed") {
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.SplitN(fields[0], ".", 2)[0]
		pkgs = append(pkgs, Package{
			Name: name, Version: fields[1], Installed: true,
		})
	}
	return pkgs, nil
}
