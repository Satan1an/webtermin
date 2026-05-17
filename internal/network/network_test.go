package network

import (
	"strings"
	"testing"
)

func TestValidIface(t *testing.T) {
	for _, s := range []string{"eth0", "enp3s0", "wlan0", "br-lan", "veth1.100"} {
		if !ValidIface(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "eth0;rm", "eth 0", strings.Repeat("a", 16), "_leading"} {
		if ValidIface(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestValidIPv4WithCIDR(t *testing.T) {
	for _, s := range []string{"10.0.0.1/24", "192.168.1.10/32", "172.16.0.0/12", "1.2.3.4/30"} {
		if !ValidIPv4WithCIDR(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "10.0.0.1", "10.0.0.1/33", "abc/24", "::1/64", "10.0.0.1/24x"} {
		if ValidIPv4WithCIDR(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestValidIP(t *testing.T) {
	for _, s := range []string{"1.1.1.1", "192.168.1.1", "::1", "2001:db8::1"} {
		if !ValidIP(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "abc", "1.2.3.4.5", "10.0.0.1/24", "$()"} {
		if ValidIP(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestValidHostname(t *testing.T) {
	for _, s := range []string{"orangepi5pro", "server-1", "host.lan", "a1.b2.example.com"} {
		if !ValidHostname(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "-leading", "trailing-", "with space", "with;semi", strings.Repeat("a", 254)} {
		if ValidHostname(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestSplitNmcliFields(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want []string
	}{
		{`eth0:ethernet:connected:Wired connection 1`, 4,
			[]string{"eth0", "ethernet", "connected", "Wired connection 1"}},
		{`wlan0:wifi:connected:My\:SSID`, 4,
			[]string{"wlan0", "wifi", "connected", `My:SSID`}},
		{`eth0:`, 2, []string{"eth0", ""}},
	}
	for _, c := range cases {
		got := splitNmcliFields(c.in, c.n)
		if len(got) != len(c.want) {
			t.Errorf("split(%q,%d) len = %d, want %d (%+v)", c.in, c.n, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("split(%q,%d)[%d] = %q, want %q", c.in, c.n, i, got[i], c.want[i])
			}
		}
	}
}
