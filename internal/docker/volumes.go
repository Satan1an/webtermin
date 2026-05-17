package docker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Volume struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	CreatedAt  string            `json:"CreatedAt"`
	Scope      string            `json:"Scope"`
	Labels     map[string]string `json:"Labels"`
	Options    map[string]string `json:"Options"`
	UsageData  *struct {
		Size     int64 `json:"Size"`
		RefCount int   `json:"RefCount"`
	} `json:"UsageData,omitempty"`
}

type volumesListResponse struct {
	Volumes  []Volume `json:"Volumes"`
	Warnings []string `json:"Warnings"`
}

// Volume names: 2..255 chars, must start with alnum, contains alnum/_/./-
var volumeNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{1,254}$`)

func ValidVolumeName(s string) bool { return volumeNameRe.MatchString(s) }

func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	var resp volumesListResponse
	err := c.request(ctx, http.MethodGet, "/volumes", nil, &resp)
	return resp.Volumes, err
}

func (c *Client) InspectVolume(ctx context.Context, name string) (*Volume, error) {
	if !ValidVolumeName(name) {
		return nil, errors.New("invalid volume name")
	}
	var out Volume
	err := c.request(ctx, http.MethodGet, "/volumes/"+url.PathEscape(name), nil, &out)
	return &out, err
}

type CreateVolumeSpec struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

func (c *Client) CreateVolume(ctx context.Context, s CreateVolumeSpec) (*Volume, error) {
	if !ValidVolumeName(s.Name) {
		return nil, errors.New("invalid volume name")
	}
	body := map[string]any{
		"Name":   s.Name,
		"Driver": defaultStr(s.Driver, "local"),
		"Labels": s.Labels,
	}
	buf, _ := json.Marshal(body)
	var out Volume
	err := c.requestJSON(ctx, "POST", "/volumes/create",
		strings.NewReader(string(buf)), &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	if !ValidVolumeName(name) {
		return errors.New("invalid volume name")
	}
	q := url.Values{}
	if force {
		q.Set("force", "1")
	}
	return c.request(ctx, http.MethodDelete,
		"/volumes/"+url.PathEscape(name)+"?"+q.Encode(), nil, nil)
}
