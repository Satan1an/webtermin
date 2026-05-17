package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

// HijackedConn is a full-duplex byte stream returned by ExecStart. Read pulls
// container stdout/stderr; Write goes to the container's stdin.
type HijackedConn interface {
	io.ReadWriteCloser
}

// ExecCreate creates an exec instance inside the running container. cmd is
// the argv to run (e.g. ["/bin/sh"]). With tty=true the stream is raw
// bidirectional; with tty=false stdout/stderr come back multiplexed.
func (c *Client) ExecCreate(ctx context.Context, containerID string, cmd []string, tty bool) (string, error) {
	if !ValidContainerID(containerID) {
		return "", errors.New("invalid container id")
	}
	if len(cmd) == 0 {
		return "", errors.New("empty cmd")
	}
	for _, a := range cmd {
		for _, r := range a {
			if r == 0 || r == '\n' {
				return "", errors.New("invalid argv")
			}
		}
	}
	body := map[string]any{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          tty,
		"Cmd":          cmd,
	}
	var out struct {
		ID string `json:"Id"`
	}
	buf, _ := json.Marshal(body)
	err := c.requestJSON(ctx, "POST",
		"/containers/"+url.PathEscape(containerID)+"/exec",
		strings.NewReader(string(buf)), &out)
	if err != nil {
		return "", err
	}
	return out.ID, nil
}

// ExecStart attaches to an exec instance and returns a hijacked TCP-style
// full-duplex stream. The HTTP client doesn't expose hijack on its response,
// so we open a fresh unix-socket connection and speak HTTP/1.1 by hand.
func (c *Client) ExecStart(ctx context.Context, execID string, tty bool) (HijackedConn, error) {
	if execID == "" {
		return nil, errors.New("empty exec id")
	}
	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	if d, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(d)
	}

	body := fmt.Sprintf(`{"Detach":false,"Tty":%v}`, tty)
	reqLines := []string{
		fmt.Sprintf("POST /%s/exec/%s/start HTTP/1.1", apiVersion, url.PathEscape(execID)),
		"Host: docker",
		"User-Agent: webtermin",
		"Content-Type: application/json",
		fmt.Sprintf("Content-Length: %d", len(body)),
		"Connection: Upgrade",
		"Upgrade: tcp",
		"",
		body,
	}
	if _, err := io.WriteString(conn, strings.Join(reqLines, "\r\n")); err != nil {
		conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	status = strings.TrimSpace(status)
	if !strings.Contains(status, " 200") && !strings.Contains(status, " 101") {
		// Drain a bit so the user gets the engine's complaint.
		extra, _ := io.ReadAll(io.LimitReader(br, 4096))
		conn.Close()
		return nil, fmt.Errorf("exec start: %s — %s", status, strings.TrimSpace(string(extra)))
	}
	// Skip headers until blank line.
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	// Reset any deadline before handing over.
	_ = conn.SetDeadline(time.Time{})
	return &hijackedConn{Conn: conn, br: br}, nil
}

// ExecResize sends a TTY resize event so the shell wraps lines correctly.
func (c *Client) ExecResize(ctx context.Context, execID string, rows, cols int) error {
	q := url.Values{}
	q.Set("h", fmt.Sprint(rows))
	q.Set("w", fmt.Sprint(cols))
	return c.request(ctx, "POST",
		"/exec/"+url.PathEscape(execID)+"/resize?"+q.Encode(), nil, nil)
}

// hijackedConn wraps the raw socket so we always Read through the bufio
// reader that may have already buffered post-header bytes.
type hijackedConn struct {
	net.Conn
	br *bufio.Reader
}

func (h *hijackedConn) Read(p []byte) (int, error) { return h.br.Read(p) }
