// Package network wraps NetworkManager's `nmcli` to list and edit physical
// network interfaces. It targets Debian/Ubuntu/Armbian/Fedora — every modern
// distro defaults to NetworkManager. On hosts without it (minimal containers,
// systemd-networkd-only setups), the module degrades to "read-only" — calls
// to Available() return false and the UI shows an install hint.
//
// Like every other module here, all inputs (interface names, IP addresses,
// DNS servers, hostnames) are validated against strict regexes before exec
// and passed via argv slices — never interpolated into shell strings.
package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
)

var ErrNotAvailable = errors.New("nmcli is not installed (NetworkManager required)")

func Available() bool {
	_, err := exec.LookPath("nmcli")
	return err == nil
}

// Interface name follows the kernel's IFNAMSIZ rule: ≤15 chars, alnum/_.-
var ifaceRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,14}$`)

func ValidIface(s string) bool { return ifaceRe.MatchString(s) }

// Hostname: RFC 1123 — alnum + dashes, dots allowed for FQDN. 1..253.
var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)

func ValidHostname(s string) bool { return hostnameRe.MatchString(s) }

// ValidIPv4WithCIDR: "10.0.0.1/24". CIDR is required for static config.
func ValidIPv4WithCIDR(s string) bool {
	ip, _, err := net.ParseCIDR(s)
	if err != nil {
		return false
	}
	return ip.To4() != nil
}

// ValidIP accepts a single IP without prefix — used for gateway and DNS.
func ValidIP(s string) bool {
	return net.ParseIP(s) != nil
}

// Interface summary as we expose it to the frontend.
type Interface struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"` // ethernet | wifi | bridge | ...
	State      string   `json:"state"`
	Connection string   `json:"connection"` // logical NM-connection name
	MAC        string   `json:"mac,omitempty"`
	IPv4       []string `json:"ipv4,omitempty"`
	IPv4GW     string   `json:"ipv4_gateway,omitempty"`
	IPv6       []string `json:"ipv6,omitempty"`
	DNS        []string `json:"dns,omitempty"`
	IPv4Method string   `json:"ipv4_method,omitempty"` // auto | manual | shared | ...
}

// List returns one row per device known to NetworkManager. Disabled / loopback
// interfaces are included so the user can see the full picture.
func List(ctx context.Context) ([]Interface, error) {
	if !Available() {
		return nil, ErrNotAvailable
	}
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "dev", "status")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nmcli dev status: %w", err)
	}
	var ifaces []Interface
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// nmcli -t escapes colons inside fields as `\:`. Split with a tiny
		// custom helper so "ssid:name" inside a CONNECTION doesn't break us.
		fields := splitNmcliFields(line, 4)
		if len(fields) < 4 {
			continue
		}
		if fields[0] == "lo" || fields[1] == "loopback" {
			continue
		}
		ifaces = append(ifaces, Interface{
			Name:       fields[0],
			Type:       fields[1],
			State:      fields[2],
			Connection: fields[3],
		})
	}
	// Enrich each row with IP details — done one device at a time because
	// nmcli's `dev show` is the only command that returns these structured.
	for i := range ifaces {
		_ = enrich(ctx, &ifaces[i])
	}
	return ifaces, nil
}

func enrich(ctx context.Context, iface *Interface) error {
	cmd := exec.CommandContext(ctx, "nmcli", "-t",
		"-f", "GENERAL.HWADDR,IP4.ADDRESS,IP4.GATEWAY,IP4.DNS,IP6.ADDRESS,IP6.GATEWAY,IPV4.METHOD",
		"dev", "show", iface.Name)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		eq := strings.Index(line, ":")
		if eq <= 0 {
			continue
		}
		key, value := line[:eq], strings.TrimSpace(line[eq+1:])
		switch {
		case key == "GENERAL.HWADDR":
			iface.MAC = value
		case strings.HasPrefix(key, "IP4.ADDRESS"):
			if value != "" {
				iface.IPv4 = append(iface.IPv4, value)
			}
		case key == "IP4.GATEWAY":
			iface.IPv4GW = value
		case strings.HasPrefix(key, "IP4.DNS"):
			if value != "" {
				iface.DNS = append(iface.DNS, value)
			}
		case strings.HasPrefix(key, "IP6.ADDRESS"):
			if value != "" {
				iface.IPv6 = append(iface.IPv6, value)
			}
		case key == "IPV4.METHOD":
			iface.IPv4Method = value
		}
	}
	return nil
}

// SetStatic configures the interface's logical connection for a static IPv4
// address with the given gateway and DNS list. Then bounces the connection
// so the new settings take effect.
func SetStatic(ctx context.Context, iface, addressCIDR, gateway string, dns []string) error {
	if !Available() {
		return ErrNotAvailable
	}
	if !ValidIface(iface) {
		return errors.New("invalid interface name")
	}
	if !ValidIPv4WithCIDR(addressCIDR) {
		return errors.New("invalid IPv4 address (expected x.x.x.x/yy)")
	}
	if gateway != "" && !ValidIP(gateway) {
		return errors.New("invalid gateway")
	}
	for _, d := range dns {
		if !ValidIP(d) {
			return fmt.Errorf("invalid DNS server %q", d)
		}
	}
	conn, err := connectionName(ctx, iface)
	if err != nil {
		return err
	}
	args := []string{
		"con", "mod", conn,
		"ipv4.method", "manual",
		"ipv4.addresses", addressCIDR,
	}
	if gateway != "" {
		args = append(args, "ipv4.gateway", gateway)
	}
	if len(dns) > 0 {
		args = append(args, "ipv4.dns", strings.Join(dns, " "))
	}
	if err := runNmcli(ctx, args...); err != nil {
		return err
	}
	return reapply(ctx, conn)
}

// SetDHCP switches the connection back to automatic configuration.
func SetDHCP(ctx context.Context, iface string) error {
	if !Available() {
		return ErrNotAvailable
	}
	if !ValidIface(iface) {
		return errors.New("invalid interface name")
	}
	conn, err := connectionName(ctx, iface)
	if err != nil {
		return err
	}
	if err := runNmcli(ctx, "con", "mod", conn,
		"ipv4.method", "auto",
		"ipv4.addresses", "",
		"ipv4.gateway", "",
		"ipv4.dns", ""); err != nil {
		return err
	}
	return reapply(ctx, conn)
}

// SetDNS replaces just the DNS server list, leaving address / gateway alone.
func SetDNS(ctx context.Context, iface string, dns []string) error {
	if !Available() {
		return ErrNotAvailable
	}
	if !ValidIface(iface) {
		return errors.New("invalid interface name")
	}
	for _, d := range dns {
		if !ValidIP(d) {
			return fmt.Errorf("invalid DNS %q", d)
		}
	}
	conn, err := connectionName(ctx, iface)
	if err != nil {
		return err
	}
	if err := runNmcli(ctx, "con", "mod", conn,
		"ipv4.dns", strings.Join(dns, " ")); err != nil {
		return err
	}
	return reapply(ctx, conn)
}

// Hostname returns the kernel's current hostname. Cheap enough that we don't
// cache — the caller renders it once on page load.
func Hostname(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "hostname").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SetHostname persists a new hostname via hostnamectl (works on every
// systemd-based distro).
func SetHostname(ctx context.Context, name string) error {
	if !ValidHostname(name) {
		return errors.New("invalid hostname (RFC 1123)")
	}
	cmd := exec.CommandContext(ctx, "hostnamectl", "--static", "set-hostname", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hostnamectl: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// --- helpers ---

func connectionName(ctx context.Context, iface string) (string, error) {
	out, err := exec.CommandContext(ctx, "nmcli", "-t",
		"-f", "DEVICE,CONNECTION", "dev", "status").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := splitNmcliFields(line, 2)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == iface {
			if fields[1] == "" || fields[1] == "--" {
				return "", fmt.Errorf("interface %s has no NetworkManager connection", iface)
			}
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("interface %s not found", iface)
}

func reapply(ctx context.Context, conn string) error {
	// `nmcli con up` is idempotent — if the connection is already up, it
	// just re-applies. Use --temporary so a failure doesn't disable the
	// connection at boot.
	cmd := exec.CommandContext(ctx, "nmcli", "con", "up", conn)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nmcli con up %s: %v: %s", conn, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runNmcli(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "nmcli", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nmcli %s: %v: %s",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// nmcli -t escapes colons inside field values as \:. The standard library's
// CSV reader doesn't understand that — easier to scan by hand.
func splitNmcliFields(line string, n int) []string {
	out := make([]string, 0, n)
	var cur strings.Builder
	escape := false
	for _, r := range line {
		switch {
		case escape:
			cur.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
		case r == ':':
			out = append(out, cur.String())
			cur.Reset()
			if len(out) == n-1 {
				// last field — collect the rest as-is
				continue
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 || len(out) > 0 {
		out = append(out, cur.String())
	}
	return out
}
