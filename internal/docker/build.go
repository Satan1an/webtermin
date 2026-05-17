package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Build streams the engine's image-build progress. ctx is the tarball context
// (typically Dockerfile + supporting files). tag is the image name to apply.
// Returns the engine's event stream — caller decodes one JSON document per
// log line and bridges it onto the wire.
func (c *Client) Build(ctx context.Context, tarball io.Reader, tag, dockerfile string) (io.ReadCloser, error) {
	q := url.Values{}
	if tag != "" {
		if !ValidImageRef(tag) {
			return nil, fmt.Errorf("invalid tag %q", tag)
		}
		q.Set("t", tag)
	}
	if dockerfile != "" {
		// Inside the tarball, Dockerfile location relative to root. Default is
		// "Dockerfile"; allow overrides like "build/Dockerfile.prod".
		q.Set("dockerfile", dockerfile)
	}
	// rm=1 cleans up intermediate containers (matches `docker build` default).
	q.Set("rm", "1")
	// nocache=0 — we want layer caching, that's why builds are fast.

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url("/build?"+q.Encode()), tarball)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("build: %d %s", res.StatusCode, string(buf))
	}
	return res.Body, nil
}
