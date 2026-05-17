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
