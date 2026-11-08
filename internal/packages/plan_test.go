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
