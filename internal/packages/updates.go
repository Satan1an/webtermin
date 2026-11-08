package packages

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Update is one upgradable package.
type Update struct {
	Name      string `json:"name"`
	Current   string `json:"current"`
	Candidate string `json:"candidate"`
	Security  bool   `json:"security"`
}

// Updates summarises what `apt upgrade` / `dnf upgrade` would bring in.
type Updates struct {
	Total     int       `json:"total"`
	Security  int       `json:"security"`
	Packages  []Update  `json:"packages"`
	CheckedAt time.Time `json:"checked_at"`
	Checking  bool      `json:"checking"` // a refresh is running in the background
	Error     string    `json:"error,omitempty"`
}

const updatesMaxAge = time.Hour

var updates struct {
	mu         sync.Mutex
	val        Updates
	refreshing bool
}

// GetUpdates returns the cached list and starts a background refresh when it
// is missing or older than an hour. It never blocks on `apt-get update`, so the
// dashboard stays fast; callers poll while Checking is true.
func GetUpdates() Updates {
	updates.mu.Lock()
	defer updates.mu.Unlock()
	if !updates.refreshing && time.Since(updates.val.CheckedAt) > updatesMaxAge && Detect() != Unknown {
		updates.refreshing = true
		go refreshUpdates()
	}
	v := updates.val
	v.Packages = append([]Update(nil), updates.val.Packages...)
	v.Checking = updates.refreshing
	return v
}

// invalidateUpdates forces the next GetUpdates to re-check, e.g. after an
// install or upgrade changed what is pending.
func invalidateUpdates() {
	updates.mu.Lock()
	updates.val.CheckedAt = time.Time{}
	updates.mu.Unlock()
}

func refreshUpdates() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var list []Update
	var err error
	switch Detect() {
	case APT:
		refreshAptLists(ctx)
		var out []byte
		out, err = exec.CommandContext(ctx, "apt", "list", "--upgradable").Output()
		list = parseAptUpgradable(string(out))
	case DNF:
		list, err = dnfUpgradable(ctx)
	}
	v := Updates{Packages: list, Total: len(list), CheckedAt: time.Now()}
	for _, u := range list {
		if u.Security {
			v.Security++
		}
	}
	if err != nil {
		v.Error = err.Error()
	}
	updates.mu.Lock()
	updates.val = v
	updates.refreshing = false
	updates.mu.Unlock()
}

// parseAptUpgradable reads `apt list --upgradable`:
//
//	libc6/stable-security 2.41-12 arm64 [upgradable from: 2.41-11]
//	docker-ce/trixie 5:29.9.3-1~debian.13~trixie arm64 [upgradable from: 5:29.9.1-1~debian.13~trixie]
func parseAptUpgradable(out string) []Update {
	var list []Update
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		slash := strings.IndexByte(line, '/')
		if slash <= 0 || !strings.Contains(line, "[upgradable from: ") {
			continue
		}
		f := strings.Fields(line[slash+1:])
		if len(f) < 2 {
			continue
		}
		cur := line[strings.Index(line, "[upgradable from: ")+len("[upgradable from: "):]
		list = append(list, Update{
			Name:      line[:slash],
			Candidate: f[1],
			Current:   strings.TrimSuffix(cur, "]"),
			Security:  strings.Contains(f[0], "security"),
		})
	}
	return list
}

// dnfUpgradable combines `dnf check-update` (exit 100 = updates available)
// with the security advisories list to flag security fixes.
func dnfUpgradable(ctx context.Context) ([]Update, error) {
	out, err := exec.CommandContext(ctx, "dnf", "-q", "check-update").Output()
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 100 {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	secOut, _ := exec.CommandContext(ctx, "dnf", "-q", "updateinfo", "list", "--security", "--updates").Output()
	return parseDnfCheckUpdate(string(out), string(secOut)), nil
}

// parseDnfCheckUpdate reads `dnf check-update` ("openssl.x86_64  1:3.2.2-6.el9  baseos")
// and marks names that appear in `dnf updateinfo list --security` output
// ("RHSA-2024:1234 Important/Sec. openssl-1:3.2.2-6.el9.x86_64").
func parseDnfCheckUpdate(out, security string) []Update {
	var secNEVRAs []string
	for _, line := range strings.Split(security, "\n") {
		if f := strings.Fields(line); len(f) >= 3 {
			secNEVRAs = append(secNEVRAs, f[len(f)-1])
		}
	}
	var list []Update
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) != 3 || !strings.Contains(f[0], ".") {
			continue
		}
		name := f[0][:strings.LastIndexByte(f[0], '.')]
		u := Update{Name: name, Candidate: f[1]}
		for _, n := range secNEVRAs {
			// "openssl-1:3…" matches openssl, but "openssl-libs-1:3…" must not.
			if rest, ok := strings.CutPrefix(n, name+"-"); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
				u.Security = true
				break
			}
		}
		list = append(list, u)
	}
	return list
}
