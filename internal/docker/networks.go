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

type Network struct {
	ID         string            `json:"Id"`
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Scope      string            `json:"Scope"`
	IPAM       NetworkIPAM       `json:"IPAM"`
	Internal   bool              `json:"Internal"`
	Attachable bool              `json:"Attachable"`
	Labels     map[string]string `json:"Labels"`
	Containers map[string]struct {
		Name        string `json:"Name"`
		EndpointID  string `json:"EndpointID"`
		IPv4Address string `json:"IPv4Address"`
	} `json:"Containers"`
}

type NetworkIPAM struct {
	Driver string `json:"Driver"`
	Config []struct {
		Subnet  string `json:"Subnet"`
		Gateway string `json:"Gateway"`
	} `json:"Config"`
}

// Names follow `[a-zA-Z0-9][a-zA-Z0-9_.-]*`, length 1..64. Same shape as
// container names but volumes/networks have their own validators because
// the engine treats reserved names (host, bridge, none) specially.
var networkNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

func ValidNetworkName(s string) bool { return networkNameRe.MatchString(s) }

func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	var out []Network
	err := c.request(ctx, http.MethodGet, "/networks", nil, &out)
	return out, err
}

func (c *Client) InspectNetwork(ctx context.Context, id string) (*Network, error) {
	if !ValidNetworkName(id) && !ValidContainerID(id) {
		return nil, errors.New("invalid network id or name")
	}
	var out Network
	err := c.request(ctx, http.MethodGet, "/networks/"+url.PathEscape(id), nil, &out)
	return &out, err
}

type CreateNetworkSpec struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver,omitempty"` // default bridge
	Internal   bool              `json:"internal,omitempty"`
	Attachable bool              `json:"attachable,omitempty"`
	Subnet     string            `json:"subnet,omitempty"` // optional CIDR
	Gateway    string            `json:"gateway,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

func (c *Client) CreateNetwork(ctx context.Context, s CreateNetworkSpec) (string, error) {
	if !ValidNetworkName(s.Name) {
		return "", errors.New("invalid network name")
	}
	// `host`, `bridge`, `none` are reserved by Docker — don't let users clobber them.
	switch s.Name {
	case "host", "bridge", "none":
		return "", errors.New("name is reserved by Docker")
	}
	body := map[string]any{
		"Name":       s.Name,
		"Driver":     defaultStr(s.Driver, "bridge"),
		"Internal":   s.Internal,
		"Attachable": s.Attachable,
		"Labels":     s.Labels,
	}
	if s.Subnet != "" || s.Gateway != "" {
		body["IPAM"] = map[string]any{
			"Driver": "default",
			"Config": []map[string]string{
				{"Subnet": s.Subnet, "Gateway": s.Gateway},
			},
		}
	}
	buf, _ := json.Marshal(body)
	var out struct {
		ID      string `json:"Id"`
		Warning string `json:"Warning"`
	}
	err := c.requestJSON(ctx, "POST", "/networks/create",
		strings.NewReader(string(buf)), &out)
	if err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	if !ValidNetworkName(id) && !ValidContainerID(id) {
		return errors.New("invalid network id or name")
	}
	return c.request(ctx, http.MethodDelete, "/networks/"+url.PathEscape(id), nil, nil)
}

func defaultStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
