package packages

import (
	"slices"
	"strings"
	"testing"
)

func TestParseAptSim(t *testing.T) {
	out := `NOTE: This is only a simulation!
Reading package lists...
Remv ifupdown [0.8.44]
Inst libndp0 (1.9-1 Debian:13/stable [arm64])
Inst network-manager (1.52.0-1 Debian:13/stable [arm64])
Inst libc6 [2.41-11] (2.41-12 Debian-Security:13/stable-security [arm64])
Conf libndp0 (1.9-1 Debian:13/stable [arm64])
`
	p := parseAptSim(out)
	if !slices.Equal(p.Install, []string{"libndp0", "network-manager"}) {
		t.Errorf("install = %v", p.Install)
	}
	if !slices.Equal(p.Upgrade, []string{"libc6"}) {
		t.Errorf("upgrade = %v", p.Upgrade)
	}
	if !slices.Equal(p.Remove, []string{"ifupdown"}) {
		t.Errorf("remove = %v", p.Remove)
	}
}

func TestParseDnfTransaction(t *testing.T) {
	out := `Dependencies resolved.
================================================================
 Package          Arch     Version            Repository   Size
================================================================
Removing:
 docker-ce        x86_64   3:27.0-1.el9       @docker      90 M
Removing dependent packages:
 docker-compose-plugin x86_64 2.29-1.el9      @docker      60 M
Upgrading:
 openssl          x86_64   1:3.2.2-6.el9      baseos      1.4 M

Transaction Summary
================================================================
Remove  2 Packages
Operation aborted.
`
	p := parseDnfTransaction(out)
	if !slices.Equal(p.Remove, []string{"docker-ce", "docker-compose-plugin"}) {
		t.Errorf("remove = %v", p.Remove)
	}
	if !slices.Equal(p.Upgrade, []string{"openssl"}) {
		t.Errorf("upgrade = %v", p.Upgrade)
	}
	if len(p.Install) != 0 {
		t.Errorf("install = %v", p.Install)
	}
}

func TestAssess_BlocksDangerousRemovals(t *testing.T) {
	sys := sysInfo{
		protected: map[string]bool{"bash": true},
		kernelPkg: "linux-image-current-rockchip64",
		selfPkg:   "webtermin",
		active:    map[string]bool{"NetworkManager": true},
		freeBytes: 10 << 30,
	}
	blocked := []string{
		"openssh-server", "sudo", "bash", "webtermin", "linux-image-current-rockchip64",
		"linux-u-boot-orangepi5pro-current", "armbian-bsp-cli-orangepi5pro", "network-manager",
	}
	for _, n := range blocked {
		if removalBlock(n, sys) == "" {
			t.Errorf("removing %s should be blocked", n)
		}
	}
	for _, n := range []string{"nginx", "ufw", "ifupdown", "netplan.io", "linux-image-6.1.0-old"} {
		if why := removalBlock(n, sys); why != "" {
			t.Errorf("removing %s should be allowed, got %q", n, why)
		}
	}
}

func TestAssess_Warnings(t *testing.T) {
	sys := sysInfo{active: map[string]bool{"systemd-networkd": true, "docker": true}, freeBytes: 10 << 30}

	p := &Plan{Install: []string{"libndp0", "network-manager"}}
	assess(p, sys)
	if len(p.Blocked) != 0 || len(p.Warnings) == 0 || !strings.Contains(p.Warnings[0], "NetworkManager") {
		t.Errorf("network-manager over networkd should warn, got %+v", p)
	}

	p = &Plan{Remove: []string{"docker-ce"}}
	assess(p, sys)
	if !slices.ContainsFunc(p.Warnings, func(w string) bool { return strings.Contains(w, "container") }) {
		t.Errorf("removing running docker should warn, got %v", p.Warnings)
	}

	p = &Plan{Install: []string{"ufw"}}
	assess(p, sys)
	if len(p.Blocked)+len(p.Warnings) != 0 {
		t.Errorf("plain install should be clean, got %+v", p)
	}
}

func TestAssess_LowDiskBlocksInstall(t *testing.T) {
	sys := sysInfo{active: map[string]bool{}, freeBytes: 100 << 20}
	p := &Plan{Install: []string{"ufw"}}
	assess(p, sys)
	if len(p.Blocked) != 1 {
		t.Fatalf("install with 100 MB free should be blocked, got %+v", p)
	}
	p = &Plan{Remove: []string{"nginx"}}
	assess(p, sys)
	if len(p.Blocked) != 0 {
		t.Errorf("removal frees space and must not be blocked by low disk, got %+v", p.Blocked)
	}
}

func TestParseAptUpgradable(t *testing.T) {
	out := `Listing...
libc6/stable-security 2.41-12 arm64 [upgradable from: 2.41-11]
docker-ce/trixie 5:29.9.3-1~debian.13~trixie arm64 [upgradable from: 5:29.9.1-1~debian.13~trixie]
`
	u := parseAptUpgradable(out)
	if len(u) != 2 || u[0].Name != "libc6" || !u[0].Security || u[0].Current != "2.41-11" || u[0].Candidate != "2.41-12" {
		t.Fatalf("libc6 parsed wrong: %+v", u)
	}
	if u[1].Name != "docker-ce" || u[1].Security {
		t.Errorf("docker-ce parsed wrong: %+v", u[1])
	}
}

func TestParseDnfCheckUpdate(t *testing.T) {
	out := "\nopenssl.x86_64   1:3.2.2-6.el9   baseos\nopenssl-libs.x86_64   1:3.2.2-6.el9   baseos\ncurl.x86_64  7.76.1-30.el9  baseos\n"
	sec := "RHSA-2024:1 Important/Sec. openssl-libs-1:3.2.2-6.el9.x86_64\n"
	u := parseDnfCheckUpdate(out, sec)
	if len(u) != 3 {
		t.Fatalf("want 3 updates, got %+v", u)
	}
	if u[0].Security || !u[1].Security || u[2].Security {
		t.Errorf("security flags wrong: %+v", u)
	}
}

func TestProgressBufferSplitsCarriageReturns(t *testing.T) {
	var b progressBuf
	b.start("install", []string{"htop"})
	_, _ = b.Write([]byte("Get:1 htop\r0% [Working]\r50% [1 htop]\nSetting up htop"))
	b.end(nil)
	if got := b.cur.Lines; len(got) != 4 || got[3] != "Setting up htop" || b.cur.Running {
		t.Errorf("lines = %q running=%v", got, b.cur.Running)
	}
}

func TestProgressBufferCollapsesPercentCounters(t *testing.T) {
	var b progressBuf
	b.start("install", nil)
	_, _ = b.Write([]byte("(Reading database ... 5%\r(Reading database ... 50%\r(Reading database ... 100%\r(Reading database ... 9363 files.)\nUnpacking htop\n"))
	want := []string{"(Reading database ... 100%", "(Reading database ... 9363 files.)", "Unpacking htop"}
	if got := b.cur.Lines; !slices.Equal(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}
