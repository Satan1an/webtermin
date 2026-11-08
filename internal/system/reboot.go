package system

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// RebootStatus says whether the host is running stale code that only a reboot
// picks up — most often a kernel installed by an upgrade.
type RebootStatus struct {
	Required      bool     `json:"required"`
	Reasons       []string `json:"reasons"`
	RunningKernel string   `json:"running_kernel"`
	NewestKernel  string   `json:"newest_kernel,omitempty"`
}

// GetRebootStatus combines three signals, since no single one covers every
// distro: Debian's /run/reboot-required flag (written by package hooks), a
// newer kernel image in /boot than the one running (Armbian's kernel packages
// don't always set the flag), and `needs-restarting -r` on Fedora/RHEL.
func GetRebootStatus(ctx context.Context) RebootStatus {
	st := RebootStatus{RunningKernel: runningKernel()}
	if _, err := os.Stat("/run/reboot-required"); err == nil {
		reason := "a package update asked for a reboot"
		if b, err := os.ReadFile("/run/reboot-required.pkgs"); err == nil {
			if pkgs := uniqueFields(string(b)); len(pkgs) > 0 {
				reason += ": " + strings.Join(pkgs, ", ")
			}
		}
		st.Reasons = append(st.Reasons, reason)
	}
	if newest := newestKernel("/boot"); newest != "" && st.RunningKernel != "" && newest != st.RunningKernel {
		st.NewestKernel = newest
		st.Reasons = append(st.Reasons, "kernel "+newest+" is installed, "+st.RunningKernel+" is running")
	}
	if _, err := exec.LookPath("needs-restarting"); err == nil {
		if ee, ok := exec.CommandContext(ctx, "needs-restarting", "-r").Run().(*exec.ExitError); ok && ee.ExitCode() == 1 {
			st.Reasons = append(st.Reasons, "needs-restarting reports core libraries or the kernel were updated")
		}
	}
	st.Required = len(st.Reasons) > 0
	return st
}

// Reboot asks systemd to reboot the host.
func Reboot(ctx context.Context) error {
	return exec.CommandContext(ctx, "systemctl", "reboot").Run()
}

func runningKernel() string {
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// newestKernel returns the release of the newest /boot/vmlinuz-<release> by
// file mtime (dpkg/rpm keep the package build time). That beats version
// sorting: kernel flavours (rockchip64, cloud-amd64, …) don't compare
// meaningfully, and the newest build is what the bootloader will pick.
func newestKernel(boot string) string {
	matches, _ := filepath.Glob(filepath.Join(boot, "vmlinuz-*"))
	type k struct {
		rel string
		mod int64
	}
	var ks []k
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil || fi.IsDir() {
			continue
		}
		rel := strings.TrimPrefix(filepath.Base(m), "vmlinuz-")
		if strings.HasSuffix(rel, ".old") || strings.HasSuffix(rel, ".bak") {
			continue
		}
		ks = append(ks, k{rel, fi.ModTime().UnixNano()})
	}
	if len(ks) == 0 {
		return ""
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i].mod > ks[j].mod })
	return ks[0].rel
}

func uniqueFields(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Fields(s) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}
