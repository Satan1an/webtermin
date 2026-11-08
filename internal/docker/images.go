package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PullImage streams progress events as `image-create` runs. Caller drains the
// reader for {"status":"...", "id":"...", "progressDetail":...} JSON docs.
func (c *Client) PullImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	if !ValidImageRef(ref) {
		return nil, errors.New("invalid image reference")
	}
	// Split into image + tag (default :latest if absent).
	image, tag := splitImageRef(ref)
	q := url.Values{}
	q.Set("fromImage", image)
	if tag != "" {
		q.Set("tag", tag)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url("/images/create?"+q.Encode()), nil)
	if err != nil {
		return nil, err
	}
	// Empty X-Registry-Auth is the documented "no creds" form; including it
	// avoids quirks on some engine versions.
	req.Header.Set("X-Registry-Auth", "e30=")
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("pull %s: %d %s", ref, res.StatusCode, strings.TrimSpace(string(buf)))
	}
	return res.Body, nil
}

// HasImage reports whether ref is present locally.
func (c *Client) HasImage(ctx context.Context, ref string) bool {
	if !ValidImageRef(ref) {
		return false
	}
	return c.request(ctx, http.MethodGet, "/images/"+url.PathEscape(ref)+"/json", nil, nil) == nil
}

// EnsureImage pulls ref unless it is already present (compose's default
// `pull_policy: missing`). The engine reports pull failures inside the
// progress stream with HTTP 200, so the stream is scanned for an error doc.
func (c *Client) EnsureImage(ctx context.Context, ref string) error {
	if c.HasImage(ctx, ref) {
		return nil
	}
	body, err := c.PullImage(ctx, ref)
	if err != nil {
		return err
	}
	defer body.Close()
	dec := json.NewDecoder(body)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("pull %s: %w", ref, err)
		}
		if msg.Error != "" {
			return fmt.Errorf("pull %s: %s", ref, msg.Error)
		}
	}
}

// RemoveImage deletes an image by id or `repo:tag`.
func (c *Client) RemoveImage(ctx context.Context, ref string, force bool) error {
	if !ValidImageRef(ref) && !ValidContainerID(ref) /* image ids share the hex shape */ {
		return errors.New("invalid image reference")
	}
	q := url.Values{}
	if force {
		q.Set("force", "1")
	}
	return c.request(ctx, http.MethodDelete,
		"/images/"+url.PathEscape(ref)+"?"+q.Encode(), nil, nil)
}

// splitImageRef breaks "nginx:1.27" → ("nginx", "1.27"). If no tag is given
// or a digest is used, returns ("", "").
func splitImageRef(s string) (image, tag string) {
	// digest reference: name@sha256:xxxx — pass through as one piece
	if strings.Contains(s, "@") {
		return s, ""
	}
	// Tag is whatever comes after the last `:` UNLESS it's a port in a
	// registry hostname (which would precede a `/`). Walk back from end.
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, ""
	}
	if strings.Contains(s[i:], "/") {
		// the colon belongs to a registry:port prefix, not a tag
		return s, ""
	}
	return s[:i], s[i+1:]
}
