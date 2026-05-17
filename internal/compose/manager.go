package compose

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Satan1an/webtermin/internal/docker"
)

// Manager turns a parsed compose File into running containers (and back).
type Manager struct {
	D *docker.Client
}

func NewManager(c *docker.Client) *Manager { return &Manager{D: c} }

// Deploy creates/updates networks, volumes, and containers for the given
// stack. If a service's container already exists, it's removed and recreated
// — partial diff-based updates are deferred to a later release.
//
// Returns the IDs of containers we ended up with, in service-start order.
func (m *Manager) Deploy(ctx context.Context, stackName string, file *File) ([]string, error) {
	if !ValidStackName(stackName) {
		return nil, errors.New("invalid stack name")
	}

	// 1. Materialise networks: each `networks: { foo: {...} }` becomes a real
	//    docker network named `<stack>_<foo>`. Already-existing ones are
	//    left untouched.
	for name, n := range file.Networks {
		if n.External {
			continue
		}
		netName := stackName + "_" + name
		if _, err := m.D.InspectNetwork(ctx, netName); err == nil {
			continue
		}
		if _, err := m.D.CreateNetwork(ctx, docker.CreateNetworkSpec{
			Name:       netName,
			Driver:     n.Driver,
			Internal:   n.Internal,
			Attachable: n.Attachable,
			Labels:     map[string]string{"webtermin.stack": stackName},
		}); err != nil {
			return nil, fmt.Errorf("create network %s: %w", netName, err)
		}
	}

	// 2. Materialise volumes: same logic.
	for name, v := range file.Volumes {
		if v.External {
			continue
		}
		volName := stackName + "_" + name
		if _, err := m.D.InspectVolume(ctx, volName); err == nil {
			continue
		}
		if _, err := m.D.CreateVolume(ctx, docker.CreateVolumeSpec{
			Name:   volName,
			Driver: v.Driver,
			Labels: map[string]string{"webtermin.stack": stackName},
		}); err != nil {
			return nil, fmt.Errorf("create volume %s: %w", volName, err)
		}
	}

	// 3. Walk services in dependency order. depends_on is honoured at start
	//    time by the toposort. Cycles are detected and surfaced.
	order, err := topoSort(file.Services)
	if err != nil {
		return nil, err
	}

	// 4. Remove any existing containers for this stack — simpler than diff.
	if err := m.RemoveContainers(ctx, stackName, true); err != nil {
		return nil, err
	}

	// 5. Create + start each service. Container name is `<stack>_<service>`,
	//    bind-mount sources that look like named volumes get prefixed too.
	containerIDs := make([]string, 0, len(order))
	for _, serviceName := range order {
		svc := file.Services[serviceName]
		spec, err := svc.ToSpec(stackName, serviceName)
		if err != nil {
			return nil, fmt.Errorf("service %s: %w", serviceName, err)
		}
		// Re-map any volume-source name to the namespaced volume we created.
		for i, mt := range spec.Mounts {
			if mt.Type != "volume" {
				continue
			}
			if _, declared := file.Volumes[mt.Source]; declared {
				if !file.Volumes[mt.Source].External {
					spec.Mounts[i].Source = stackName + "_" + mt.Source
				}
			}
		}
		id, err := m.D.CreateContainer(ctx, spec)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", serviceName, err)
		}
		containerIDs = append(containerIDs, id)
	}
	return containerIDs, nil
}

// ListContainers returns the live containers that belong to the named stack.
// Uses the `webtermin.stack` label as the filter key.
func (m *Manager) ListContainers(ctx context.Context, stackName string) ([]docker.Container, error) {
	all, err := m.D.ListContainers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]docker.Container, 0)
	for _, c := range all {
		if c.Labels["webtermin.stack"] == stackName {
			out = append(out, c)
		}
	}
	return out, nil
}

// Start brings every container in the stack up (idempotent).
func (m *Manager) Start(ctx context.Context, stackName string) error {
	cs, err := m.ListContainers(ctx, stackName)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.State == "running" {
			continue
		}
		if err := m.D.DoAction(ctx, c.ID, docker.ActionStart); err != nil {
			return fmt.Errorf("start %s: %w", containerLabel(c), err)
		}
	}
	return nil
}

// Stop brings every container in the stack down (idempotent).
func (m *Manager) Stop(ctx context.Context, stackName string) error {
	cs, err := m.ListContainers(ctx, stackName)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.State != "running" && c.State != "paused" {
			continue
		}
		if err := m.D.DoAction(ctx, c.ID, docker.ActionStop); err != nil {
			return fmt.Errorf("stop %s: %w", containerLabel(c), err)
		}
	}
	return nil
}

// RemoveContainers force-removes every container belonging to the stack.
// If keepVolumes is false the underlying networks/volumes we created stay.
func (m *Manager) RemoveContainers(ctx context.Context, stackName string, force bool) error {
	cs, err := m.ListContainers(ctx, stackName)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if err := m.D.RemoveContainer(ctx, c.ID, force); err != nil {
			return fmt.Errorf("remove %s: %w", containerLabel(c), err)
		}
	}
	return nil
}

// RemoveStack tears down containers + (optionally) any networks/volumes that
// were declared in the compose file and aren't external. Named volumes that
// the user explicitly chose to keep can be preserved with removeData=false.
func (m *Manager) RemoveStack(ctx context.Context, stackName string, file *File, removeData bool) error {
	if err := m.RemoveContainers(ctx, stackName, true); err != nil {
		return err
	}
	for name, n := range file.Networks {
		if n.External {
			continue
		}
		_ = m.D.RemoveNetwork(ctx, stackName+"_"+name) // ignore — may not exist
	}
	if removeData {
		for name, v := range file.Volumes {
			if v.External {
				continue
			}
			_ = m.D.RemoveVolume(ctx, stackName+"_"+name, true)
		}
	}
	return nil
}

func containerLabel(c docker.Container) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	return c.ID[:12]
}

// topoSort orders services so that depended-upon services come first. Returns
// the order. Cycles surface as an explicit error.
func topoSort(services map[string]Service) ([]string, error) {
	names := make([]string, 0, len(services))
	for n := range services {
		names = append(names, n)
	}
	sort.Strings(names)

	indeg := map[string]int{}
	dependents := map[string][]string{}
	for _, n := range names {
		indeg[n] = 0
	}
	for _, n := range names {
		for _, dep := range services[n].DependsOn {
			if _, ok := services[dep]; !ok {
				continue // depends_on a service that isn't in this compose — tolerate
			}
			indeg[n]++
			dependents[dep] = append(dependents[dep], n)
		}
	}
	var queue []string
	for _, n := range names {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	var out []string
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		out = append(out, n)
		ds := dependents[n]
		sort.Strings(ds)
		for _, d := range ds {
			indeg[d]--
			if indeg[d] == 0 {
				queue = append(queue, d)
			}
		}
	}
	if len(out) != len(names) {
		return nil, errors.New("compose: depends_on cycle detected")
	}
	return out, nil
}
