package packages

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"
)

// Plan is what a package operation would do to the system, computed by a dry
// run (`apt-get -s` / `dnf --assumeno`) and checked against the host before
// anything is executed.
type Plan struct {
	Install  []string  `json:"install"`
	Upgrade  []string  `json:"upgrade"`
	Remove   []string  `json:"remove"`
	Blocked  []Blocked `json:"blocked"`  // non-empty: the panel refuses to run it
	Warnings []string  `json:"warnings"` // shown to the admin before confirming
}

type Blocked struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var (
	ErrBusy         = errors.New("another package operation is already running")
	ErrBlocked      = errors.New("operation blocked by safety checks")
	ErrNeedsConfirm = errors.New("operation removes packages that were not confirmed")
)

// opMu serialises mutating operations: two concurrent apt-get runs would just
// fight over the dpkg lock, and a plan computed during another install is stale.
var opMu = make(chan struct{}, 1)

const (
	opTimeout    = 30 * time.Minute
	minFreeBytes = 500 << 20
	listsMaxAge  = 24 * time.Hour
)

// coreReasons are packages the panel never removes, whatever the dry run says.
// Essential/required packages are detected from dpkg on top of this list.
var coreReasons = map[string]string{
	"openssh-server": "removing it cuts SSH access to the host",
	"sudo":           "removing it locks admins out of privilege escalation",
	"apt":            "removing it breaks package management",
	"dpkg":           "removing it breaks package management",
	"dnf":            "removing it breaks package management",
	"rpm":            "removing it breaks package management",
	"systemd":        "removing it breaks the init system",
	"systemd-sysv":   "removing it breaks the init system",
	"libc6":          "removing it breaks every program on the host",
	"glibc":          "removing it breaks every program on the host",
}

// bootPrefixes cover the board/bootloader packages on Armbian and friends.
var bootPrefixes = []string{"linux-u-boot-", "linux-dtb-", "armbian-bsp-", "armbian-firmware", "grub-", "shim-"}

// sysInfo is the host state the safety checks need. Collected once per plan so
// the rules themselves stay pure and testable.
type sysInfo struct {
	protected  map[string]bool // Essential: yes / Priority: required
	kernelPkg  string          // owns the running kernel image
	selfPkg    string          // owns the webtermin binary
	active     map[string]bool // systemd units that are running
	netplanCfg bool            // /etc/netplan has config files
	freeBytes  uint64          // free space under /var
}

// MakePlan dry-runs op ("install", "remove", "upgrade") and applies the
// safety rules. It never changes the system, apart from refreshing stale apt
// lists so the dry run matches what a real install would fetch.
func MakePlan(ctx context.Context, op string, names ...string) (*Plan, error) {
	for _, n := range names {
		if !ValidName(n) {
			return nil, fmt.Errorf("invalid package name %q", n)
		}
	}
	var p *Plan
	var err error
	switch Detect() {
	case APT:
		if op != "remove" {
			refreshAptLists(ctx)
		}
		p, err = aptPlan(ctx, op, names)
	case DNF:
		p, err = dnfPlan(ctx, op, names)
	default:
		return nil, errors.New("no supported package manager found")
	}
	if err != nil {
		return nil, err
	}
	assess(p, collectSysInfo(ctx))
	return p, nil
}

// Execute re-plans, refuses blocked operations and unconfirmed removals, then
// runs the real command. The caller's context only bounds the planning: once
// dpkg starts, killing it half way leaves the package database broken, so the
// run is detached from the HTTP request and bounded by opTimeout instead.
func Execute(ctx context.Context, op string, names, confirmRemove []string) (*Plan, error) {
	select {
	case opMu <- struct{}{}:
		defer func() { <-opMu }()
	default:
		return nil, ErrBusy
	}
	p, err := MakePlan(ctx, op, names...)
	if err != nil {
		return nil, err
	}
	if len(p.Blocked) > 0 {
		return p, ErrBlocked
	}
	for _, n := range p.Remove {
		if !slices.Contains(confirmRemove, n) {
			return p, ErrNeedsConfirm
		}
	}
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opTimeout)
	defer cancel()
	if len(names) == 0 && op == "upgrade" {
		op = "upgrade-all"
	}
	return p, run(runCtx, op, len(p.Remove) > 0, names...)
}

func assess(p *Plan, sys sysInfo) {
	for _, n := range p.Remove {
		if why := removalBlock(n, sys); why != "" {
			p.Blocked = append(p.Blocked, Blocked{Name: n, Reason: why})
		}
	}
	if sys.freeBytes > 0 && sys.freeBytes < minFreeBytes && len(p.Install)+len(p.Upgrade) > 0 {
		p.Blocked = append(p.Blocked, Blocked{Reason: fmt.Sprintf(
			"only %d MB free under /var; dpkg can fail half way and leave packages broken", sys.freeBytes>>20)})
	}

	if slices.Contains(p.Install, "network-manager") || slices.Contains(p.Install, "NetworkManager") {
		if !sys.active["NetworkManager"] && (sys.active["systemd-networkd"] || sys.active["networking"] || sys.netplanCfg) {
			p.Warnings = append(p.Warnings,
				"The network is currently managed by systemd-networkd/ifupdown/netplan. NetworkManager may take over "+
					"unconfigured interfaces and the host can drop off the network. Have console access ready.")
		}
	}
	if sys.active["docker"] {
		for _, n := range p.Remove {
			if n == "docker-ce" || n == "docker.io" || n == "containerd.io" || n == "containerd" {
				p.Warnings = append(p.Warnings, "Docker is running: every container on this host will stop.")
				break
			}
		}
	}
	for _, n := range append(slices.Clone(p.Install), p.Upgrade...) {
		if strings.HasPrefix(n, "linux-image-") || strings.HasPrefix(n, "kernel") {
			p.Warnings = append(p.Warnings, "A kernel package changes: reboot to use it, and keep console access in case it does not boot.")
			break
		}
	}
	if len(p.Remove) > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d package(s) will be removed.", len(p.Remove)))
	}
}

func removalBlock(name string, sys sysInfo) string {
	if why, ok := coreReasons[name]; ok {
		return why
	}
	if name == sys.selfPkg {
		return "it is webtermin itself; the panel would stop mid-operation"
	}
	if name == sys.kernelPkg {
		return "it provides the running kernel"
	}
	if sys.protected[name] {
		return "essential/required system package"
	}
	for _, pre := range bootPrefixes {
		if strings.HasPrefix(name, pre) {
			return "bootloader/board package; removing it can leave the host unbootable"
		}
	}
	switch {
	case name == "network-manager" || name == "NetworkManager":
		if sys.active["NetworkManager"] {
			return "NetworkManager manages the network right now; removing it takes the host offline"
		}
	case name == "ifupdown":
		if sys.active["networking"] {
			return "ifupdown manages the network right now; removing it takes the host offline"
		}
	case name == "netplan.io" || name == "netplan-generator":
		if sys.netplanCfg {
			return "netplan holds the network configuration"
		}
	}
	return ""
}

// --- dry runs ---

func aptPlan(ctx context.Context, op string, names []string) (*Plan, error) {
	var args []string
	switch op {
	case "install":
		args = append([]string{"-s", "install"}, names...)
	case "remove":
		args = append([]string{"-s", "remove"}, names...)
	case "upgrade":
		if len(names) == 0 {
			args = []string{"-s", "upgrade"}
		} else {
			args = append([]string{"-s", "install", "--only-upgrade"}, names...)
		}
	default:
		return nil, fmt.Errorf("unsupported op %q", op)
	}
	out, err := exec.CommandContext(ctx, "apt-get", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("apt-get dry run: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return parseAptSim(string(out)), nil
}

// parseAptSim reads `apt-get -s` output:
//
//	Inst network-manager (1.52.0-1 Debian:13/stable [arm64])
//	Inst libc6 [2.41-11] (2.41-12 Debian-Security:13/stable-security [arm64])
//	Remv ifupdown [0.8.44]
//
// An Inst line with a bracketed old version is an upgrade.
func parseAptSim(out string) *Plan {
	p := &Plan{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "Inst":
			if len(f) > 2 && strings.HasPrefix(f[2], "[") {
				p.Upgrade = append(p.Upgrade, f[1])
			} else {
				p.Install = append(p.Install, f[1])
			}
		case "Remv":
			p.Remove = append(p.Remove, f[1])
		}
	}
	return p
}

func dnfPlan(ctx context.Context, op string, names []string) (*Plan, error) {
	if op != "install" && op != "remove" && op != "upgrade" {
		return nil, fmt.Errorf("unsupported op %q", op)
	}
	// --assumeno prints the transaction and exits 1, so the exit code is
	// meaningless here; a parse with no sections means dnf failed.
	out, _ := exec.CommandContext(ctx, "dnf", append([]string{"--assumeno", op}, names...)...).CombinedOutput()
	p := parseDnfTransaction(string(out))
	if len(p.Install)+len(p.Upgrade)+len(p.Remove) == 0 && !strings.Contains(string(out), "Nothing to do") {
		return nil, fmt.Errorf("dnf dry run: %s", strings.TrimSpace(string(out)))
	}
	return p, nil
}

// parseDnfTransaction reads the transaction table dnf prints before asking:
//
//	Installing:
//	 wireguard-tools   x86_64   1.0.20210914-6.el9   appstream   123 k
//	Removing dependent packages:
//	 foo               noarch   1.0-1                @baseos      12 k
func parseDnfTransaction(out string) *Plan {
	p := &Plan{}
	var dst *[]string
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if line[0] != ' ' {
			switch h := strings.ToLower(strings.TrimSpace(line)); {
			case strings.HasPrefix(h, "installing"):
				dst = &p.Install
			case strings.HasPrefix(h, "upgrading"), strings.HasPrefix(h, "downgrading"):
				dst = &p.Upgrade
			case strings.HasPrefix(h, "removing"), strings.HasPrefix(h, "erasing"):
				dst = &p.Remove
			default:
				dst = nil
			}
			continue
		}
		if dst == nil {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 4 && !strings.HasPrefix(f[0], "replacing") {
			*dst = append(*dst, f[0])
		}
	}
	return p
}

// --- host state ---

func collectSysInfo(ctx context.Context) sysInfo {
	sys := sysInfo{protected: map[string]bool{}, active: map[string]bool{}}
	for _, u := range []string{"NetworkManager", "systemd-networkd", "networking", "docker", "ssh", "sshd"} {
		if exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", u).Run() == nil {
			sys.active[u] = true
		}
	}
	if m, _ := os.ReadDir("/etc/netplan"); len(m) > 0 {
		sys.netplanCfg = true
	}
	var st syscall.Statfs_t
	if syscall.Statfs("/var", &st) == nil {
		sys.freeBytes = uint64(st.Bavail) * uint64(st.Bsize)
	}
	if exe, err := os.Executable(); err == nil {
		sys.selfPkg = ownerOf(ctx, exe)
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		rel := strings.TrimSpace(string(b))
		sys.kernelPkg = ownerOf(ctx, "/boot/vmlinuz-"+rel)
		if sys.kernelPkg == "" {
			sys.kernelPkg = ownerOf(ctx, "/lib/modules/"+rel+"/modules.order")
		}
	}
	if Detect() == APT {
		out, _ := exec.CommandContext(ctx, "dpkg-query", "-W", "-f", "${Package}\t${Essential}\t${Priority}\n").Output()
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Split(line, "\t")
			if len(f) == 3 && (f[1] == "yes" || f[2] == "required") {
				sys.protected[f[0]] = true
			}
		}
	}
	return sys
}

func ownerOf(ctx context.Context, path string) string {
	switch Detect() {
	case APT:
		out, err := exec.CommandContext(ctx, "dpkg-query", "-S", path).Output()
		if err != nil {
			return ""
		}
		// "linux-image-current-rockchip64: /boot/vmlinuz-6.18.60-..."
		name, _, _ := strings.Cut(strings.TrimSpace(string(out)), ":")
		return name
	case DNF:
		out, err := exec.CommandContext(ctx, "rpm", "-qf", "--qf", "%{NAME}", path).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return ""
}

// refreshAptLists runs `apt-get update` when the package cache is older than
// a day, so the dry run does not plan against versions the mirror no longer
// serves. Failures are ignored: the dry run below reports anything real.
func refreshAptLists(ctx context.Context) {
	if fi, err := os.Stat("/var/cache/apt/pkgcache.bin"); err == nil && time.Since(fi.ModTime()) < listsMaxAge {
		return
	}
	_ = exec.CommandContext(ctx, "apt-get", "update", "-q", "-o", "DPkg::Lock::Timeout=60").Run()
}
