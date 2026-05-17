package wireguard

import (
	"strings"
	"testing"
)

func TestValidIface(t *testing.T) {
	for _, s := range []string{"wg0", "wg-home", "vpn.lan"} {
		if !ValidIface(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "wg0;rm", "wg0 wg1", "way-too-long-iface-name"} {
		if ValidIface(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestGenerateKeypair(t *testing.T) {
	priv, pub, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidKey(priv) || !ValidKey(pub) {
		t.Errorf("generated keys aren't 44-char base64: priv=%q pub=%q", priv, pub)
	}
	if priv == pub {
		t.Fatal("private and public keys should differ")
	}
	priv2, _, _ := GenerateKeypair()
	if priv == priv2 {
		t.Fatal("two GenerateKeypair calls returned the same private key")
	}
}

func TestValidCIDRList(t *testing.T) {
	for _, s := range []string{"10.0.0.5/32", "10.0.0.0/24,192.168.1.0/24", "::1/128"} {
		if !validCIDRList(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "10.0.0.5", "10.0.0.5/64", "abc"} {
		if validCIDRList(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestValidEndpoint(t *testing.T) {
	for _, s := range []string{"vpn.example.com:51820", "1.2.3.4:51820", "10.0.0.1:443"} {
		if !validEndpoint(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range []string{"", "novpn", "host;port:1234", "host:abc", "rm -rf"} {
		if validEndpoint(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func TestParseDump(t *testing.T) {
	// `wg show <iface> dump` uses TAB-separated fields. Each peer row has 8
	// fields: public preshared endpoint allowed_ips handshake rx tx keepalive.
	src := strings.Join([]string{
		"PRIVATEKEYREDACTED==========================\tPUBKEYAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\t51820\toff",
		"PEER1PUBKEY===================================\t(none)\t(none)\t10.0.0.2/32\t1700000000\t1024\t2048\t25",
		"PEER2PUBKEY===================================\t(none)\t1.2.3.4:9999\t10.0.0.3/32\t0\t0\t0\toff",
	}, "\n")
	st := parseDump("wg0", src)
	if st.Iface != "wg0" || st.Port != 51820 {
		t.Errorf("header: %+v", st)
	}
	if len(st.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(st.Peers))
	}
	if st.Peers[0].Endpoint != "" {
		t.Errorf("(none) endpoint should be empty, got %q", st.Peers[0].Endpoint)
	}
	if st.Peers[0].AllowedIPs != "10.0.0.2/32" {
		t.Errorf("peer[0].allowed_ips: %q", st.Peers[0].AllowedIPs)
	}
	if st.Peers[0].LatestHandshake != 1700000000 {
		t.Errorf("peer[0].handshake: %d", st.Peers[0].LatestHandshake)
	}
}

func TestDropPeerBlock(t *testing.T) {
	src := `[Interface]
Address = 10.0.0.1/24
ListenPort = 51820
PrivateKey = SERVERPRIV==================================

[Peer]
# Name = laptop
PublicKey = KEEPMEKEEPMEKEEPMEKEEPMEKEEPMEKEEPMEKEEPME=
AllowedIPs = 10.0.0.2/32

[Peer]
# Name = phone
PublicKey = REMOVEMEREMOVEMEREMOVEMEREMOVEMEREMOVEMEREM=
AllowedIPs = 10.0.0.3/32`

	out := dropPeerBlock(src, "REMOVEMEREMOVEMEREMOVEMEREMOVEMEREMOVEMEREM=")
	if strings.Contains(out, "REMOVEME") {
		t.Errorf("removed peer should not appear in output:\n%s", out)
	}
	if !strings.Contains(out, "KEEPME") {
		t.Errorf("non-matching peer should be preserved:\n%s", out)
	}
}

func TestClientConfigRender(t *testing.T) {
	cfg := ClientConfig{
		ClientPrivateKey: "CLIENTPRIV==================================",
		ClientAddress:    "10.0.0.5/32",
		ServerPublicKey:  "SERVERPUB===================================",
		ServerEndpoint:   "vpn.example.com:51820",
		AllowedIPs:       "10.0.0.0/24",
		DNS:              "1.1.1.1",
	}
	got := cfg.String()
	for _, must := range []string{
		"[Interface]", "PrivateKey = CLIENTPRIV",
		"Address = 10.0.0.5/32", "DNS = 1.1.1.1",
		"[Peer]", "PublicKey = SERVERPUB",
		"Endpoint = vpn.example.com:51820",
		"AllowedIPs = 10.0.0.0/24",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("rendered config missing %q:\n%s", must, got)
		}
	}
}
