// Package docker talks to the Docker engine HTTP API over the local Unix
// socket at /var/run/docker.sock. We deliberately avoid the official
// `docker/docker` Go client — it pulls hundreds of MB of dependencies for
// what is, for our needs, half a dozen REST endpoints.
package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	sockPath = "/var/run/docker.sock"
	// "v1.43" works against any modern Docker (~20.10+) engine. We use the
	// versioned prefix to insulate ourselves from compat shifts.
	apiVersion = "v1.43"
)

var ErrNotAvailable = errors.New("docker socket not available")

// Client is a small HTTP client wired to a Unix socket. Safe for concurrent use.
type Client struct {
	hc *http.Client
}

// New returns a Client if /var/run/docker.sock exists and accepts connections.
// Returns ErrNotAvailable otherwise — callers should propagate this to the UI.
func New() (*Client, error) {
	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		return nil, ErrNotAvailable
	}
	_ = conn.Close()
	return &Client{
		hc: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", sockPath)
				},
			},
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *Client) url(path string) string {
	// http+unix scheme is not a real one — we just need a syntactically
	// valid URL. The actual transport ignores the host.
	return "http://docker/" + apiVersion + path
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return err
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		return fmt.Errorf("docker %s %s: %d %s", method, path, res.StatusCode, strings.TrimSpace(string(buf)))
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

// requestJSON is request() for callers that already have a JSON body.
// Sets Content-Type: application/json automatically.
func (c *Client) requestJSON(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		return fmt.Errorf("docker %s %s: %d %s", method, path, res.StatusCode, strings.TrimSpace(string(buf)))
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

// Container summarises a row of `docker ps -a` / `containers/json?all=1`.
type Container struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	Command string            `json:"Command"`
	Created int64             `json:"Created"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Ports   []ContainerPort   `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
}

type ContainerPort struct {
	IP          string `json:"IP,omitempty"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort,omitempty"`
	Type        string `json:"Type"`
}

// ListContainers returns every container, running or stopped.
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	var out []Container
	err := c.request(ctx, http.MethodGet, "/containers/json?all=1", nil, &out)
	return out, err
}

type Image struct {
	ID          string            `json:"Id"`
	RepoTags    []string          `json:"RepoTags"`
	Created     int64             `json:"Created"`
	Size        int64             `json:"Size"`
	VirtualSize int64             `json:"VirtualSize"`
	Labels      map[string]string `json:"Labels"`
}

func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	var out []Image
	err := c.request(ctx, http.MethodGet, "/images/json", nil, &out)
	return out, err
}

// Allowed actions on a container.
type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionPause   Action = "pause"
	ActionUnpause Action = "unpause"
	ActionKill    Action = "kill"
)

func ValidAction(a string) bool {
	switch Action(a) {
	case ActionStart, ActionStop, ActionRestart, ActionPause, ActionUnpause, ActionKill:
		return true
	}
	return false
}

// ValidContainerID enforces Docker's id format — 12+ hex chars — so we never
// interpolate junk into a URL path.
func ValidContainerID(id string) bool {
	if len(id) < 12 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// DoAction performs the action against id. Returns immediately on success;
// the docker daemon does the actual state transition asynchronously.
func (c *Client) DoAction(ctx context.Context, id string, action Action) error {
	if !ValidContainerID(id) {
		return errors.New("invalid container id")
	}
	if !ValidAction(string(action)) {
		return errors.New("invalid action")
	}
	return c.request(ctx, http.MethodPost,
		"/containers/"+url.PathEscape(id)+"/"+string(action), nil, nil)
}

// Inspect returns the raw JSON document `docker inspect` shows. We don't
// type the whole thing (it's huge); callers can decode the fields they need.
func (c *Client) Inspect(ctx context.Context, id string) (map[string]any, error) {
	if !ValidContainerID(id) {
		return nil, errors.New("invalid container id")
	}
	var out map[string]any
	err := c.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, &out)
	return out, err
}

// CreateContainerSpec is the subset of the Docker engine's container-create
// API we expose. It's intentionally minimal — just what a Portainer-style
// "Add container" form fills in.
type CreateContainerSpec struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Cmd           []string          `json:"cmd,omitempty"`
	Env           []string          `json:"env,omitempty"` // KEY=VALUE
	WorkingDir    string            `json:"working_dir,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"` // no, always, unless-stopped, on-failure
	Privileged    bool              `json:"privileged,omitempty"`
	Tty           bool              `json:"tty,omitempty"`
	OpenStdin     bool              `json:"open_stdin,omitempty"`
	NetworkMode   string            `json:"network_mode,omitempty"`
	PortBindings  []PortBinding     `json:"port_bindings,omitempty"`
	Mounts        []MountSpec       `json:"mounts,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	AutoStart     bool              `json:"auto_start,omitempty"`
}

// PortBinding: publish containerPort to the host on hostIP:hostPort/proto.
type PortBinding struct {
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      string `json:"host_port"` // string per engine API; numeric like "8080"
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp | udp
}

// MountSpec: a bind mount or named volume.
type MountSpec struct {
	Type     string `json:"type"` // bind | volume
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// ValidImageRef accepts the docker image reference shape: lower-case
// `[registry/]repo[:tag|@digest]`. We don't try to be perfect — engine
// will reject anything we miss — we just block obvious junk like spaces,
// shell metacharacters, and traversal.
var imageRefRe = regexp.MustCompile(`^[a-z0-9._:/@-]{1,255}$`)

func ValidImageRef(s string) bool {
	return imageRefRe.MatchString(s) && !strings.Contains(s, "..")
}

// ValidContainerName: optional leading slash, then a-z A-Z 0-9 _ . -, 2-128 chars
// (the engine's own validation regex from the upstream code).
var containerNameRe = regexp.MustCompile(`^/?[a-zA-Z0-9][a-zA-Z0-9_.-]{1,127}$`)

func ValidContainerName(s string) bool { return s == "" || containerNameRe.MatchString(s) }

// ValidRestartPolicy: one of the documented values, or empty for default.
func ValidRestartPolicy(s string) bool {
	switch s {
	case "", "no", "always", "unless-stopped", "on-failure":
		return true
	}
	return false
}

// CreateContainer maps our concise spec onto the engine's busy JSON body and
// posts /containers/create. Returns the new container id on success.
func (c *Client) CreateContainer(ctx context.Context, s CreateContainerSpec) (string, error) {
	if !ValidImageRef(s.Image) {
		return "", errors.New("invalid image reference")
	}
	if !ValidContainerName(s.Name) {
		return "", errors.New("invalid container name")
	}
	if !ValidRestartPolicy(s.RestartPolicy) {
		return "", errors.New("invalid restart policy")
	}
	for _, p := range s.PortBindings {
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			return "", errors.New("port protocol must be tcp or udp")
		}
		if p.ContainerPort <= 0 || p.ContainerPort > 65535 {
			return "", errors.New("invalid container port")
		}
		if p.HostPort != "" {
			if !portStringRe.MatchString(p.HostPort) {
				return "", errors.New("invalid host port")
			}
		}
	}
	for _, m := range s.Mounts {
		if m.Type != "bind" && m.Type != "volume" {
			return "", errors.New("mount type must be bind or volume")
		}
		if m.Target == "" || strings.Contains(m.Target, "..") || !strings.HasPrefix(m.Target, "/") {
			return "", errors.New("invalid mount target")
		}
		if m.Type == "bind" && (m.Source == "" || strings.Contains(m.Source, "..") || !strings.HasPrefix(m.Source, "/")) {
			return "", errors.New("invalid bind source")
		}
		if m.Type == "volume" && !volumeNameRe.MatchString(m.Source) {
			return "", errors.New("invalid volume source name")
		}
	}

	body := buildCreateBody(s)
	q := url.Values{}
	if s.Name != "" {
		q.Set("name", strings.TrimPrefix(s.Name, "/"))
	}
	buf, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url("/containers/create?"+q.Encode()), strings.NewReader(string(buf)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("create: %d %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	var resp struct {
		ID       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return "", err
	}
	if s.AutoStart {
		if err := c.DoAction(ctx, resp.ID, ActionStart); err != nil {
			return resp.ID, fmt.Errorf("created %s but failed to start: %w", resp.ID, err)
		}
	}
	return resp.ID, nil
}

// portStringRe accepts a single numeric port (no ranges from the UI yet).
var portStringRe = regexp.MustCompile(`^\d{1,5}$`)

// buildCreateBody flattens our spec into the verbose engine schema.
func buildCreateBody(s CreateContainerSpec) map[string]any {
	exposed := map[string]struct{}{}
	portBindings := map[string][]map[string]string{}
	for _, p := range s.PortBindings {
		key := fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
		exposed[key] = struct{}{}
		portBindings[key] = append(portBindings[key], map[string]string{
			"HostIp":   p.HostIP,
			"HostPort": p.HostPort,
		})
	}
	mounts := make([]map[string]any, 0, len(s.Mounts))
	for _, m := range s.Mounts {
		mounts = append(mounts, map[string]any{
			"Type":     m.Type,
			"Source":   m.Source,
			"Target":   m.Target,
			"ReadOnly": m.ReadOnly,
		})
	}
	hostConfig := map[string]any{
		"Mounts":       mounts,
		"PortBindings": portBindings,
		"Privileged":   s.Privileged,
		"NetworkMode":  s.NetworkMode,
	}
	if s.RestartPolicy != "" && s.RestartPolicy != "no" {
		hostConfig["RestartPolicy"] = map[string]any{"Name": s.RestartPolicy}
	}
	body := map[string]any{
		"Image":        s.Image,
		"Env":          s.Env,
		"Cmd":          s.Cmd,
		"WorkingDir":   s.WorkingDir,
		"Tty":          s.Tty,
		"OpenStdin":    s.OpenStdin,
		"Labels":       s.Labels,
		"ExposedPorts": exposed,
		"HostConfig":   hostConfig,
	}
	return body
}

// RemoveContainer deletes a stopped container. If force is true, a running
// container is killed first.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	if !ValidContainerID(id) {
		return errors.New("invalid container id")
	}
	q := url.Values{}
	if force {
		q.Set("force", "1")
	}
	q.Set("v", "1") // also remove anonymous volumes the container created
	return c.request(ctx, http.MethodDelete,
		"/containers/"+url.PathEscape(id)+"?"+q.Encode(), nil, nil)
}

// Stats returns a ReadCloser streaming one JSON document per ~1s. Cancel ctx
// to stop.
func (c *Client) Stats(ctx context.Context, id string) (io.ReadCloser, error) {
	if !ValidContainerID(id) {
		return nil, errors.New("invalid container id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.url("/containers/"+url.PathEscape(id)+"/stats?stream=1"), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("stats: %d %s", res.StatusCode, strings.TrimSpace(string(buf)))
	}
	return res.Body, nil
}

// Logs streams demultiplexed stdout+stderr for a container. The returned
// reader yields plain UTF-8 text; close it to stop streaming. tail=N tail lines.
func (c *Client) Logs(ctx context.Context, id string, tail int) (io.ReadCloser, error) {
	if !ValidContainerID(id) {
		return nil, errors.New("invalid container id")
	}
	if tail <= 0 || tail > 5000 {
		tail = 200
	}
	q := url.Values{}
	q.Set("stdout", "1")
	q.Set("stderr", "1")
	q.Set("follow", "1")
	q.Set("tail", fmt.Sprintf("%d", tail))
	q.Set("timestamps", "0")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.url("/containers/"+url.PathEscape(id)+"/logs?"+q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("logs: %d %s", res.StatusCode, strings.TrimSpace(string(buf)))
	}
	return &logDemuxer{r: res.Body, br: bufio.NewReader(res.Body)}, nil
}

// logDemuxer strips Docker's 8-byte log-frame headers and yields plain text.
// Frame format: [STREAM,0,0,0,SIZE0,SIZE1,SIZE2,SIZE3][payload]
type logDemuxer struct {
	r   io.ReadCloser
	br  *bufio.Reader
	buf []byte
	pos int
}

func (d *logDemuxer) Read(p []byte) (int, error) {
	if d.pos >= len(d.buf) {
		var hdr [8]byte
		if _, err := io.ReadFull(d.br, hdr[:]); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(hdr[4:8])
		if size == 0 {
			return 0, nil
		}
		if cap(d.buf) < int(size) {
			d.buf = make([]byte, size)
		} else {
			d.buf = d.buf[:size]
		}
		if _, err := io.ReadFull(d.br, d.buf); err != nil {
			return 0, err
		}
		d.pos = 0
	}
	n := copy(p, d.buf[d.pos:])
	d.pos += n
	return n, nil
}

func (d *logDemuxer) Close() error { return d.r.Close() }
