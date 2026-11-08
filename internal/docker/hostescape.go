package docker

import "strings"

// The panel talks to the Docker engine as root, so some container settings
// are equivalent to a root shell on the host. These helpers name them, so the
// server can keep them admin-only while operators still manage ordinary
// containers.

// HostEscape reports why creating the container would hand over the host, or
// "" when it wouldn't. Named volumes are safe (CreateVolumeSpec exposes no
// driver options, so a volume can't be pointed at a host path).
func (s CreateContainerSpec) HostEscape() string {
	if s.Privileged {
		return "privileged mode"
	}
	if s.NetworkMode == "host" {
		return "host network mode"
	}
	for _, m := range s.Mounts {
		if m.Type != "volume" {
			return "bind mount of host path " + m.Source
		}
	}
	return ""
}

// dangerousCaps are capabilities that let a process inside a container reach
// the host.
var dangerousCaps = map[string]bool{
	"ALL": true, "SYS_ADMIN": true, "SYS_MODULE": true, "SYS_PTRACE": true,
	"SYS_RAWIO": true, "DAC_READ_SEARCH": true,
}

// InspectHostEscape is HostEscape for an existing container, judged from its
// `docker inspect` JSON (the container may have been created outside the
// panel, so it checks more than the panel itself can set).
func InspectHostEscape(info map[string]any) string {
	hc, _ := info["HostConfig"].(map[string]any)
	if p, _ := hc["Privileged"].(bool); p {
		return "privileged mode"
	}
	for _, ns := range []string{"NetworkMode", "PidMode", "IpcMode"} {
		if v, _ := hc[ns].(string); v == "host" {
			return "host " + strings.TrimSuffix(ns, "Mode") + " namespace"
		}
	}
	if devs, _ := hc["Devices"].([]any); len(devs) > 0 {
		return "host devices"
	}
	caps, _ := hc["CapAdd"].([]any)
	for _, c := range caps {
		name, _ := c.(string)
		if dangerousCaps[strings.TrimPrefix(strings.ToUpper(name), "CAP_")] {
			return "capability " + name
		}
	}
	mounts, _ := info["Mounts"].([]any)
	for _, m := range mounts {
		mm, _ := m.(map[string]any)
		if t, _ := mm["Type"].(string); t == "bind" {
			src, _ := mm["Source"].(string)
			return "bind mount of host path " + src
		}
	}
	return ""
}
