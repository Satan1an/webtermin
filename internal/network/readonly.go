package network

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Manager names what owns the network configuration when NetworkManager is
// absent. The panel only edits through nmcli, so for these it shows a
// read-only view instead of pushing the admin to install NetworkManager on a
// host that already works (which can take the box offline).
func Manager(ctx context.Context) string {
	active := func(unit string) bool {
		return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit).Run() == nil
	}
	switch {
	case Available() && active("NetworkManager"):
		return "NetworkManager"
	case active("systemd-networkd"):
		if m, _ := os.ReadDir("/etc/netplan"); len(m) > 0 {
			return "netplan (systemd-networkd)"
		}
		return "systemd-networkd"
	case active("networking"):
		return "ifupdown"
	}
	return "unknown"
}

// ipAddr / ipRoute mirror the bits of `ip -j addr` / `ip -j route` we read.
type ipAddr struct {
	IfName    string   `json:"ifname"`
	OperState string   `json:"operstate"`
	LinkType  string   `json:"link_type"`
	Address   string   `json:"address"`
	Flags     []string `json:"flags"`
	AddrInfo  []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		PrefixLen int    `json:"prefixlen"`
		Scope     string `json:"scope"`
		Dynamic   bool   `json:"dynamic"`
	} `json:"addr_info"`
}

type ipRoute struct {
	Dst     string `json:"dst"`
	Gateway string `json:"gateway"`
	Dev     string `json:"dev"`
}

// ListReadOnly builds the interface list from iproute2, which every distro
// ships. DNS comes from resolv.conf (system-wide, so it's shown on every
// interface with a default route).
func ListReadOnly(ctx context.Context) ([]Interface, error) {
	addrOut, err := exec.CommandContext(ctx, "ip", "-j", "addr").Output()
	if err != nil {
		return nil, fmt.Errorf("ip addr: %w", err)
	}
	routeOut, _ := exec.CommandContext(ctx, "ip", "-j", "route", "show", "default").Output()
	resolv, _ := os.ReadFile("/etc/resolv.conf")
	return parseIPJSON(addrOut, routeOut, string(resolv))
}

func parseIPJSON(addrOut, routeOut []byte, resolv string) ([]Interface, error) {
	var addrs []ipAddr
	if err := json.Unmarshal(addrOut, &addrs); err != nil {
		return nil, fmt.Errorf("parse ip addr: %w", err)
	}
	var routes []ipRoute
	_ = json.Unmarshal(routeOut, &routes)
	gw := map[string]string{}
	for _, r := range routes {
		if r.Gateway != "" && gw[r.Dev] == "" {
			gw[r.Dev] = r.Gateway
		}
	}
	var dns []string
	for _, line := range strings.Split(resolv, "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "nameserver" {
			dns = append(dns, f[1])
		}
	}

	var out []Interface
	for _, a := range addrs {
		if a.LinkType == "loopback" || fallbackDevs[a.IfName] || strings.HasPrefix(a.IfName, "veth") {
			continue
		}
		it := Interface{
			Name:  a.IfName,
			Type:  linkKind(a),
			State: strings.ToLower(a.OperState),
			MAC:   a.Address,
		}
		method := ""
		for _, ai := range a.AddrInfo {
			if ai.Scope != "global" {
				continue
			}
			cidr := fmt.Sprintf("%s/%d", ai.Local, ai.PrefixLen)
			if ai.Family == "inet" {
				it.IPv4 = append(it.IPv4, cidr)
				if method == "" {
					method = "manual"
				}
				if ai.Dynamic {
					method = "auto"
				}
			} else if ai.Family == "inet6" {
				it.IPv6 = append(it.IPv6, cidr)
			}
		}
		it.IPv4Method = method
		if g, ok := gw[a.IfName]; ok {
			it.IPv4GW = g
			it.DNS = dns
		}
		out = append(out, it)
	}
	return out, nil
}

// fallbackDevs are the placeholder tunnels the kernel creates when a tunnel
// module loads; they are never configured and only clutter the list. veth
// pairs (one per container) are skipped for the same reason.
var fallbackDevs = map[string]bool{
	"tunl0": true, "gre0": true, "gretap0": true, "erspan0": true, "ip_vti0": true,
	"ip6_vti0": true, "ip6tnl0": true, "ip6gre0": true, "sit0": true,
}

func linkKind(a ipAddr) string {
	switch {
	case strings.HasPrefix(a.IfName, "wl"):
		return "wifi"
	case strings.HasPrefix(a.IfName, "wg"):
		return "wireguard"
	case strings.HasPrefix(a.IfName, "docker"), strings.HasPrefix(a.IfName, "br-"), strings.HasPrefix(a.IfName, "br"):
		return "bridge"
	case strings.HasPrefix(a.IfName, "veth"):
		return "veth"
	case a.LinkType == "ether":
		return "ethernet"
	case a.LinkType == "none":
		return "tunnel"
	}
	return a.LinkType
}
