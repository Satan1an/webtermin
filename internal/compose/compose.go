// Package compose parses a minimal-but-useful subset of docker-compose v3.x
// YAML and translates each service into a docker.CreateContainerSpec the
// existing internal/docker engine client can consume.
//
// What we support:
//
//	services:
//	  <name>:
//	    image, container_name, restart, command (string or list),
//	    working_dir, network_mode, privileged, tty, stdin_open,
//	    ports (short or long form), environment (list or map),
//	    volumes (short string form), networks (list), depends_on (list),
//	    labels (map or list)
//	networks: { <name>: { driver?, internal?, attachable? } }
//	volumes:  { <name>: { driver? } }
//
// Out of scope for v0.6 (engine will reject if used):
//
//	build, healthcheck, deploy, secrets, configs, profiles, extension fields.
package compose

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Satan1an/webtermin/internal/docker"
	"gopkg.in/yaml.v3"
)

// File is the parsed top-level compose document.
type File struct {
	Version  string                  `yaml:"version"`
	Services map[string]Service      `yaml:"services"`
	Networks map[string]NetworkBlock `yaml:"networks,omitempty"`
	Volumes  map[string]VolumeBlock  `yaml:"volumes,omitempty"`
}

type Service struct {
	Image         string         `yaml:"image"`
	ContainerName string         `yaml:"container_name,omitempty"`
	Restart       string         `yaml:"restart,omitempty"`
	Command       Cmd            `yaml:"command,omitempty"`
	Entrypoint    Cmd            `yaml:"entrypoint,omitempty"`
	WorkingDir    string         `yaml:"working_dir,omitempty"`
	NetworkMode   string         `yaml:"network_mode,omitempty"`
	Privileged    bool           `yaml:"privileged,omitempty"`
	TTY           bool           `yaml:"tty,omitempty"`
	StdinOpen     bool           `yaml:"stdin_open,omitempty"`
	Ports         []string       `yaml:"ports,omitempty"`
	Environment   StringMapOrSeq `yaml:"environment,omitempty"`
	Volumes       []string       `yaml:"volumes,omitempty"`
	Networks      StringList     `yaml:"networks,omitempty"`
	DependsOn     StringList     `yaml:"depends_on,omitempty"`
	Labels        StringMapOrSeq `yaml:"labels,omitempty"`
}

type NetworkBlock struct {
	Driver     string `yaml:"driver,omitempty"`
	Internal   bool   `yaml:"internal,omitempty"`
	Attachable bool   `yaml:"attachable,omitempty"`
	External   bool   `yaml:"external,omitempty"`
}

type VolumeBlock struct {
	Driver   string `yaml:"driver,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

// Cmd accepts either a bare string (`command: foo bar`) or a list
// (`command: [foo, bar]`). Both flow into a []string argv.
type Cmd []string

func (c *Cmd) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		// shell-split lightly: spaces only, no escapes. Docker compose's official
		// parser uses go-shlex; for our subset, this is enough.
		*c = strings.Fields(node.Value)
		return nil
	case yaml.SequenceNode:
		var s []string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*c = s
		return nil
	}
	return fmt.Errorf("compose: unexpected node kind for command/entrypoint")
}

// StringList accepts a list of strings OR a single string.
type StringList []string

func (s *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*s = []string{node.Value}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*s = list
		return nil
	case yaml.MappingNode:
		// networks: { web: {} } — flatten to keys
		var m map[string]any
		if err := node.Decode(&m); err != nil {
			return err
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		*s = keys
		return nil
	}
	return fmt.Errorf("compose: unexpected node kind for string list")
}

// StringMapOrSeq is the recurring compose convention where environment/labels
// can come as either a map ({KEY: val}) or a list (["KEY=val"]).
type StringMapOrSeq map[string]string

func (s *StringMapOrSeq) UnmarshalYAML(node *yaml.Node) error {
	out := map[string]string{}
	switch node.Kind {
	case yaml.MappingNode:
		var m map[string]string
		if err := node.Decode(&m); err != nil {
			return err
		}
		for k, v := range m {
			out[k] = v
		}
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		for _, item := range list {
			if i := strings.Index(item, "="); i >= 0 {
				out[item[:i]] = item[i+1:]
			} else {
				out[item] = ""
			}
		}
	default:
		return fmt.Errorf("compose: unexpected node kind for map/seq")
	}
	*s = out
	return nil
}

// Parse decodes the compose YAML into File and applies cross-section checks.
func Parse(yamlText string) (*File, error) {
	var f File
	if err := yaml.Unmarshal([]byte(yamlText), &f); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if len(f.Services) == 0 {
		return nil, errors.New("compose: no services declared")
	}
	for name, svc := range f.Services {
		if svc.Image == "" {
			return nil, fmt.Errorf("compose: service %q has no image (build: is not supported in v0.6)", name)
		}
		if !docker.ValidImageRef(svc.Image) {
			return nil, fmt.Errorf("compose: service %q has invalid image %q", name, svc.Image)
		}
	}
	return &f, nil
}

// Validate runs all the same checks Parse does plus naming. Standalone helper
// for the HTTP layer to surface friendly errors before a deploy attempt.
func Validate(yamlText string) error {
	_, err := Parse(yamlText)
	return err
}

// portShortRe captures `host:container[/proto]` or just `container[/proto]`.
var portShortRe = regexp.MustCompile(
	`^(?:(?P<host>\d{1,5}):)?(?P<container>\d{1,5})(?:/(?P<proto>tcp|udp))?$`)

// ParsePort accepts compose's short-form port spec and returns one binding.
func ParsePort(s string) (docker.PortBinding, error) {
	m := portShortRe.FindStringSubmatch(s)
	if m == nil {
		return docker.PortBinding{}, fmt.Errorf("compose: cannot parse port %q", s)
	}
	cp, _ := strconv.Atoi(m[portShortRe.SubexpIndex("container")])
	hp := m[portShortRe.SubexpIndex("host")]
	proto := m[portShortRe.SubexpIndex("proto")]
	if proto == "" {
		proto = "tcp"
	}
	return docker.PortBinding{
		HostPort:      hp,
		ContainerPort: cp,
		Protocol:      proto,
	}, nil
}

// ParseVolume parses one of:
//
//	./host:/container
//	/host:/container
//	/host:/container:ro
//	volume-name:/container
//	volume-name:/container:ro
func ParseVolume(s string) (docker.MountSpec, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return docker.MountSpec{}, fmt.Errorf("compose: cannot parse volume spec %q", s)
	}
	src, dst := parts[0], parts[1]
	ro := false
	if len(parts) == 3 {
		switch parts[2] {
		case "ro":
			ro = true
		case "rw":
			ro = false
		default:
			return docker.MountSpec{}, fmt.Errorf("compose: unknown mount mode %q", parts[2])
		}
	}
	t := "volume"
	if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") {
		t = "bind"
	}
	return docker.MountSpec{
		Type:     t,
		Source:   src,
		Target:   dst,
		ReadOnly: ro,
	}, nil
}

// ToSpec turns a parsed compose Service into a docker.CreateContainerSpec.
// stackName is used to prefix container_name (when not given) and to label
// the container so we can find it later.
func (svc Service) ToSpec(stackName, serviceName string) (docker.CreateContainerSpec, error) {
	containerName := svc.ContainerName
	if containerName == "" {
		containerName = stackName + "_" + serviceName
	}
	envs := make([]string, 0, len(svc.Environment))
	for k, v := range svc.Environment {
		if v == "" {
			envs = append(envs, k)
		} else {
			envs = append(envs, k+"="+v)
		}
	}
	sort.Strings(envs) // stable ordering

	ports := make([]docker.PortBinding, 0, len(svc.Ports))
	for _, p := range svc.Ports {
		pb, err := ParsePort(p)
		if err != nil {
			return docker.CreateContainerSpec{}, err
		}
		ports = append(ports, pb)
	}
	mounts := make([]docker.MountSpec, 0, len(svc.Volumes))
	for _, v := range svc.Volumes {
		mt, err := ParseVolume(v)
		if err != nil {
			return docker.CreateContainerSpec{}, err
		}
		mounts = append(mounts, mt)
	}

	// Stack labels follow docker-compose's own convention so containers play
	// nicely with `docker compose` CLI if someone uses both.
	labels := map[string]string{
		"com.docker.compose.project": stackName,
		"com.docker.compose.service": serviceName,
		"webtermin.stack":            stackName,
	}
	for k, v := range svc.Labels {
		labels[k] = v
	}

	// If a single network is named and the service joins exactly one, pass it
	// as NetworkMode. Multi-network attach in the same create call isn't
	// trivial in the engine API; we'll handle multi-attach in a v0.7 follow-up.
	networkMode := svc.NetworkMode
	if networkMode == "" && len(svc.Networks) == 1 {
		networkMode = stackName + "_" + svc.Networks[0]
	}

	return docker.CreateContainerSpec{
		Name:          containerName,
		Image:         svc.Image,
		Cmd:           []string(svc.Command),
		Env:           envs,
		WorkingDir:    svc.WorkingDir,
		RestartPolicy: normaliseRestart(svc.Restart),
		Privileged:    svc.Privileged,
		Tty:           svc.TTY,
		OpenStdin:     svc.StdinOpen,
		NetworkMode:   networkMode,
		PortBindings:  ports,
		Mounts:        mounts,
		Labels:        labels,
		AutoStart:     true,
	}, nil
}

func normaliseRestart(s string) string {
	switch s {
	case "", "no", "always", "unless-stopped", "on-failure":
		return s
	}
	// Compose also accepts "on-failure:N" — we strip the count for now and
	// fall back to plain "on-failure".
	if strings.HasPrefix(s, "on-failure") {
		return "on-failure"
	}
	return "no"
}

// LabelKey is used to find containers belonging to a stack later. Returned in
// the docker filter form `label=key=value`.
func LabelKey(stackName string) string {
	return "webtermin.stack=" + stackName
}

// ValidStackName: lower-case alnum + dash, 1–32. The same shape compose uses
// for project names, and stable enough to plumb through container names.
var stackNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func ValidStackName(s string) bool { return stackNameRe.MatchString(s) }
