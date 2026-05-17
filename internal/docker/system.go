package docker

import (
	"context"
	"errors"
	"net/http"
)

type SystemInfo struct {
	Containers        int    `json:"Containers"`
	ContainersRunning int    `json:"ContainersRunning"`
	ContainersPaused  int    `json:"ContainersPaused"`
	ContainersStopped int    `json:"ContainersStopped"`
	Images            int    `json:"Images"`
	Driver            string `json:"Driver"`
	KernelVersion     string `json:"KernelVersion"`
	OperatingSystem   string `json:"OperatingSystem"`
	OSType            string `json:"OSType"`
	Architecture      string `json:"Architecture"`
	NCPU              int    `json:"NCPU"`
	MemTotal          int64  `json:"MemTotal"`
	ServerVersion     string `json:"ServerVersion"`
	DockerRootDir     string `json:"DockerRootDir"`
}

func (c *Client) Info(ctx context.Context) (*SystemInfo, error) {
	var out SystemInfo
	err := c.request(ctx, http.MethodGet, "/info", nil, &out)
	return &out, err
}

// DiskUsage returns `docker system df` data — sizes per object kind.
func (c *Client) DiskUsage(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.request(ctx, http.MethodGet, "/system/df", nil, &out)
	return out, err
}

type PruneTarget string

const (
	PruneContainers PruneTarget = "containers"
	PruneImages     PruneTarget = "images"
	PruneVolumes    PruneTarget = "volumes"
	PruneNetworks   PruneTarget = "networks"
)

func ValidPruneTarget(s string) bool {
	switch PruneTarget(s) {
	case PruneContainers, PruneImages, PruneVolumes, PruneNetworks:
		return true
	}
	return false
}

// Prune asks the engine to remove unused objects of the given kind. The
// response is engine-defined; we just pass it through.
func (c *Client) Prune(ctx context.Context, target PruneTarget) (map[string]any, error) {
	if !ValidPruneTarget(string(target)) {
		return nil, errors.New("invalid prune target")
	}
	var out map[string]any
	err := c.request(ctx, http.MethodPost, "/"+string(target)+"/prune", nil, &out)
	return out, err
}
