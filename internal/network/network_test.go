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

func TestParseIPJSON(t *testing.T) {
	addr := []byte(`[
 {"ifname":"lo","link_type":"loopback","operstate":"UNKNOWN","addr_info":[{"family":"inet","local":"127.0.0.1","prefixlen":8,"scope":"host"}]},
 {"ifname":"end0","link_type":"ether","operstate":"UP","address":"c0:74:2b:fe:aa:01","addr_info":[
   {"family":"inet","local":"192.168.31.87","prefixlen":24,"scope":"global","dynamic":true},
   {"family":"inet6","local":"fe80::1","prefixlen":64,"scope":"link"}]},
 {"ifname":"gre0","link_type":"gre","operstate":"DOWN","addr_info":[]},
 {"ifname":"veth1a2b","link_type":"ether","operstate":"UP","addr_info":[]},
 {"ifname":"docker0","link_type":"ether","operstate":"DOWN","addr_info":[{"family":"inet","local":"172.17.0.1","prefixlen":16,"scope":"global"}]}
]`)
	route := []byte(`[{"dst":"default","gateway":"192.168.31.1","dev":"end0"}]`)
	ifs, err := parseIPJSON(addr, route, "# generated\nnameserver 192.168.31.1\nnameserver 1.1.1.1\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(ifs) != 2 {
		t.Fatalf("loopback, fallback tunnels and veths must be skipped, got %+v", ifs)
	}
	e := ifs[0]
	if e.Name != "end0" || e.Type != "ethernet" || e.State != "up" || e.IPv4Method != "auto" ||
		len(e.IPv4) != 1 || e.IPv4[0] != "192.168.31.87/24" || e.IPv4GW != "192.168.31.1" || len(e.DNS) != 2 || len(e.IPv6) != 0 {
		t.Errorf("end0 parsed wrong: %+v", e)
	}
	d := ifs[1]
	if d.Type != "bridge" || d.IPv4Method != "manual" || d.IPv4GW != "" || d.DNS != nil {
		t.Errorf("docker0 parsed wrong: %+v", d)
	}
}
