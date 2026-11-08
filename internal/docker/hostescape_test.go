package docker

import "testing"

func TestHostEscape(t *testing.T) {
	cases := []struct {
		spec   CreateContainerSpec
		escape bool
	}{
		{CreateContainerSpec{Image: "alpine"}, false},
		{CreateContainerSpec{Image: "alpine", Mounts: []MountSpec{{Type: "volume", Source: "data", Target: "/d"}}}, false},
		{CreateContainerSpec{Image: "alpine", NetworkMode: "bridge"}, false},
		{CreateContainerSpec{Image: "alpine", Privileged: true}, true},
		{CreateContainerSpec{Image: "alpine", NetworkMode: "host"}, true},
		{CreateContainerSpec{Image: "alpine", Mounts: []MountSpec{{Type: "bind", Source: "/", Target: "/h"}}}, true},
	}
	for i, c := range cases {
		if got := c.spec.HostEscape() != ""; got != c.escape {
			t.Errorf("case %d: escape=%v, want %v", i, got, c.escape)
		}
	}
}

func TestInspectHostEscape(t *testing.T) {
	hc := func(kv map[string]any) map[string]any { return map[string]any{"HostConfig": kv} }
	cases := []struct {
		info   map[string]any
		escape bool
	}{
		{hc(map[string]any{"NetworkMode": "bridge"}), false},
		{map[string]any{"HostConfig": map[string]any{}, "Mounts": []any{map[string]any{"Type": "volume"}}}, false},
		{hc(map[string]any{"Privileged": true}), true},
		{hc(map[string]any{"PidMode": "host"}), true},
		{hc(map[string]any{"Devices": []any{map[string]any{"PathOnHost": "/dev/sda"}}}), true},
		{hc(map[string]any{"CapAdd": []any{"CAP_SYS_ADMIN"}}), true},
		{hc(map[string]any{"CapAdd": []any{"NET_BIND_SERVICE"}}), false},
		{map[string]any{"HostConfig": map[string]any{}, "Mounts": []any{
			map[string]any{"Type": "bind", "Source": "/var/run/docker.sock"}}}, true},
	}
	for i, c := range cases {
		if got := InspectHostEscape(c.info) != ""; got != c.escape {
			t.Errorf("case %d: escape=%v, want %v", i, got, c.escape)
		}
	}
}
