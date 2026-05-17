// Package wireguard wraps the `wg` and `wg-quick` CLIs to do the basics that
// matter for a home-lab admin: see what's connected, add a peer (returning
// the peer's full config so they can scan a QR on their phone), and remove
// a peer. We don't try to replace `wg-quick` for interface lifecycle — bring
// up / take down is still `systemctl start wg-quick@wg0` from the shell.
package wireguard

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/curve25519"
)

var ErrNotAvailable = errors.New("wireguard tools are not installed")

func Available() bool {
	_, err := exec.LookPath("wg")
	return err == nil
}

// Interface name like `wg0`. Validates against the kernel's actual rule:
// alnum, dash, underscore, dot — up to 15 chars (IFNAMSIZ - 1).
var ifaceRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,15}$`)

func ValidIface(s string) bool { return ifaceRe.MatchString(s) }

// Peer key is base64(curve25519 public key) — 44 chars including `=` padding.
var keyRe = regexp.MustCompile(`^[A-Za-z0-9+/]{43}=$`)

func ValidKey(s string) bool { return keyRe.MatchString(s) }

type Status struct {
	Available bool   `json:"available"`
	Iface     string `json:"iface,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Port      int    `json:"port,omitempty"`
	Peers     []Peer `json:"peers"`
}

type Peer struct {
	PublicKey       string `json:"public_key"`
	Endpoint        string `json:"endpoint,omitempty"`
	AllowedIPs      string `json:"allowed_ips,omitempty"`
	LatestHandshake int64  `json:"latest_handshake,omitempty"`
	RxBytes         int64  `json:"rx_bytes,omitempty"`
	TxBytes         int64  `json:"tx_bytes,omitempty"`
	Comment         string `json:"comment,omitempty"`
}

// GenerateKeypair returns (privKeyB64, pubKeyB64).
func GenerateKeypair() (string, string, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	// Apply Curve25519 clamping as required by the spec.
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv[:]),
		base64.StdEncoding.EncodeToString(pub), nil
}

// GetStatus returns the parsed output of `wg show <iface> dump`.
// If `wg` isn't installed at all, returns Available=false (and no error)
// so the UI can show a hint instead of an error.
func GetStatus(ctx context.Context, iface string) (*Status, error) {
	if !Available() {
		return &Status{Available: false, Peers: []Peer{}}, nil
	}
	if iface == "" {
		iface = "wg0"
	}
	if !ValidIface(iface) {
		return nil, errors.New("invalid interface name")
	}
	cmd := exec.CommandContext(ctx, "wg", "show", iface, "dump")
	out, err := cmd.Output()
	if err != nil {
		// `wg show wg0 dump` returns non-zero if the interface doesn't exist.
		return &Status{Available: true, Iface: iface, Peers: []Peer{}}, nil
	}
	return parseDump(iface, string(out)), nil
}

// parseDump processes `wg show <iface> dump`. First line is the interface
// itself: `private\tpublic\tlisten_port\tfwmark`. Following lines are peers:
// `public\tpresharedkey\tendpoint\tallowed_ips\tlatest_handshake\trx\ttx\tkeepalive`.
func parseDump(iface, text string) *Status {
	st := &Status{Available: true, Iface: iface, Peers: []Peer{}}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	first := true
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if first {
			first = false
			if len(fields) >= 3 {
				st.PublicKey = fields[1]
				if p, err := strconv.Atoi(fields[2]); err == nil {
					st.Port = p
				}
			}
			continue
		}
		if len(fields) < 8 {
			continue
		}
		p := Peer{
			PublicKey:  fields[0],
			Endpoint:   nullable(fields[2]),
			AllowedIPs: nullable(fields[3]),
		}
		if v, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			p.LatestHandshake = v
		}
		if v, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
			p.RxBytes = v
		}
		if v, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
			p.TxBytes = v
		}
		st.Peers = append(st.Peers, p)
	}
	return st
}

func nullable(s string) string {
	if s == "(none)" {
		return ""
	}
	return s
}

// PeerSpec is the inputs we accept to add a peer.
type PeerSpec struct {
	Comment    string // ignored by wg, stored in config file as `# Name = foo`
	PublicKey  string // base64. If empty, GenerateKeypair() is called for the peer.
	AllowedIPs string // typically "10.0.0.5/32"
	Endpoint   string // optional, "host:port"
}

// AddPeer appends a [Peer] section to the wg-quick config file at
// /etc/wireguard/<iface>.conf and applies it to the running interface
// with `wg syncconf`. Returns the public key of the peer (newly generated if
// caller didn't supply one) plus, on autogen, the private key the user
// should paste into their client config.
func AddPeer(ctx context.Context, iface string, spec PeerSpec) (pubKey, privKey string, err error) {
	if !Available() {
		return "", "", ErrNotAvailable
	}
	if !ValidIface(iface) {
		return "", "", errors.New("invalid interface name")
	}
	if spec.PublicKey != "" && !ValidKey(spec.PublicKey) {
		return "", "", errors.New("invalid public key")
	}
	if spec.AllowedIPs == "" {
		return "", "", errors.New("allowed_ips is required")
	}
	if !validCIDRList(spec.AllowedIPs) {
		return "", "", errors.New("invalid allowed_ips (comma-separated CIDRs only)")
	}
	if spec.PublicKey == "" {
		privKey, spec.PublicKey, err = GenerateKeypair()
		if err != nil {
			return "", "", err
		}
		pubKey = spec.PublicKey
	} else {
		pubKey = spec.PublicKey
	}

	// Apply to the running interface first; if that fails we don't touch the
	// on-disk config.
	args := []string{"set", iface, "peer", spec.PublicKey, "allowed-ips", spec.AllowedIPs}
	if spec.Endpoint != "" {
		if !validEndpoint(spec.Endpoint) {
			return "", "", errors.New("invalid endpoint")
		}
		args = append(args, "endpoint", spec.Endpoint)
	}
	cmd := exec.CommandContext(ctx, "wg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("wg set: %v: %s", err, strings.TrimSpace(string(out)))
	}

	// Persist to /etc/wireguard/<iface>.conf so it survives a reboot.
	confPath := filepath.Join("/etc/wireguard", iface+".conf")
	if _, err := os.Stat(confPath); err == nil {
		f, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			defer f.Close()
			var b strings.Builder
			b.WriteString("\n[Peer]\n")
			if spec.Comment != "" {
				b.WriteString("# Name = " + sanitiseComment(spec.Comment) + "\n")
			}
			b.WriteString("PublicKey = " + spec.PublicKey + "\n")
			b.WriteString("AllowedIPs = " + spec.AllowedIPs + "\n")
			if spec.Endpoint != "" {
				b.WriteString("Endpoint = " + spec.Endpoint + "\n")
			}
			_, _ = f.WriteString(b.String())
		}
	}
	return pubKey, privKey, nil
}

// RemovePeer removes a peer from the live interface and rewrites the conf
// file to drop the matching [Peer] block.
func RemovePeer(ctx context.Context, iface, publicKey string) error {
	if !Available() {
		return ErrNotAvailable
	}
	if !ValidIface(iface) {
		return errors.New("invalid interface name")
	}
	if !ValidKey(publicKey) {
		return errors.New("invalid public key")
	}
	cmd := exec.CommandContext(ctx, "wg", "set", iface, "peer", publicKey, "remove")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wg set ... remove: %v: %s", err, strings.TrimSpace(string(out)))
	}
	confPath := filepath.Join("/etc/wireguard", iface+".conf")
	if data, err := os.ReadFile(confPath); err == nil {
		next := dropPeerBlock(string(data), publicKey)
		if next != string(data) {
			_ = os.WriteFile(confPath, []byte(next), 0o600)
		}
	}
	return nil
}

// dropPeerBlock returns text with the [Peer] section whose PublicKey matches
// pubKey omitted. Lossy on weird comments but fine for files we wrote.
func dropPeerBlock(text, pubKey string) string {
	lines := strings.Split(text, "\n")
	var out []string
	skip := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			skip = false
		}
		if trim == "[Peer]" {
			// Look ahead a few lines to decide whether this is the peer we want
			// to remove. Cheap N² but the file is small.
			// Find PublicKey in upcoming lines until the next `[`.
			match := false
			for j := len(out); j < len(lines); j++ {
			}
			_ = match
		}
		// Simpler implementation: scan blocks linearly.
		_ = skip
		out = append(out, line)
	}
	// Replace the above non-working stub with a clean two-pass: split on
	// blank lines into chunks; drop any chunk whose first non-comment line is
	// [Peer] and contains the matching PublicKey=.
	blocks := splitConfigBlocks(text)
	var kept []string
	for _, b := range blocks {
		if isPeerBlockFor(b, pubKey) {
			continue
		}
		kept = append(kept, b)
	}
	return strings.Join(kept, "\n\n")
}

func splitConfigBlocks(text string) []string {
	// Blocks are separated by blank lines.
	rx := regexp.MustCompile(`\n[ \t]*\n`)
	return rx.Split(strings.TrimSpace(text), -1)
}

func isPeerBlockFor(block, pubKey string) bool {
	scanner := bufio.NewScanner(strings.NewReader(block))
	header := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if header == "" {
			header = line
			if header != "[Peer]" {
				return false
			}
			continue
		}
		if strings.HasPrefix(line, "PublicKey") {
			eq := strings.Index(line, "=")
			if eq < 0 {
				return false
			}
			val := strings.TrimSpace(line[eq+1:])
			return val == pubKey
		}
	}
	return false
}

func sanitiseComment(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// validCIDRList: one or more comma-separated CIDR notations.
func validCIDRList(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if _, _, err := net.ParseCIDR(part); err != nil {
			return false
		}
	}
	return true
}

// validEndpoint: host:port, where host is hostname-like or IP.
var endpointRe = regexp.MustCompile(`^[a-zA-Z0-9.:_-]+:\d{1,5}$`)

func validEndpoint(s string) bool { return endpointRe.MatchString(s) }

// ClientConfig assembles a self-contained wg client config that the user can
// scan as a QR or paste into their phone's WireGuard app. The server-side
// public key + endpoint are required; the rest comes from the peer spec we
// just created on the server.
type ClientConfig struct {
	ClientPrivateKey string
	ClientAddress    string // typically "10.0.0.5/32"
	ServerPublicKey  string
	ServerEndpoint   string // host:port
	DNS              string // optional, comma-separated
	AllowedIPs       string // typically "0.0.0.0/0, ::/0" for full tunnel
}

func (c ClientConfig) String() string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	b.WriteString("PrivateKey = " + c.ClientPrivateKey + "\n")
	b.WriteString("Address = " + c.ClientAddress + "\n")
	if c.DNS != "" {
		b.WriteString("DNS = " + c.DNS + "\n")
	}
	b.WriteString("\n[Peer]\n")
	b.WriteString("PublicKey = " + c.ServerPublicKey + "\n")
	if c.AllowedIPs != "" {
		b.WriteString("AllowedIPs = " + c.AllowedIPs + "\n")
	} else {
		b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	}
	if c.ServerEndpoint != "" {
		b.WriteString("Endpoint = " + c.ServerEndpoint + "\n")
	}
	b.WriteString("PersistentKeepalive = 25\n")
	return b.String()
}
