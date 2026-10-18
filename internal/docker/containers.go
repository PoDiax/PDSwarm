package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/go-connections/nat"
)

type CreateContainerRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Env           []string          `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMount     `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
	MemoryMB      int64             `json:"memory_mb"`
	CPUCores      float64           `json:"cpu_cores"`
	Labels        map[string]string `json:"labels"`
	LogDriver     string            `json:"log_driver,omitempty"`
	LogOpts       map[string]string `json:"log_opts,omitempty"`
}

type PortMapping struct {
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"` // "tcp" or "udp"
}

type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

func (r *CreateContainerRequest) Validate() error {
	if r.Image == "" {
		return errors.New("image is required")
	}
	return nil
}

// FilterUserEnv returns only environment variables that were defined or overridden
// by the user, excluding unchanged base image defaults.
func FilterUserEnv(containerEnv []string, imageEnv []string) []string {
	if len(imageEnv) == 0 {
		res := make([]string, len(containerEnv))
		copy(res, containerEnv)
		return res
	}

	imgMap := make(map[string]string, len(imageEnv))
	for _, e := range imageEnv {
		k, v, _ := strings.Cut(e, "=")
		imgMap[k] = v
	}

	userEnv := make([]string, 0, len(containerEnv))
	for _, e := range containerEnv {
		k, v, _ := strings.Cut(e, "=")
		baseVal, exists := imgMap[k]
		if !exists || baseVal != v {
			userEnv = append(userEnv, e)
		}
	}
	return userEnv
}

// ContainerToCreateRequest extracts configuration parameters from an inspected container
// and converts them into a CreateContainerRequest suitable for cloning or recreating.
func ContainerToCreateRequest(c types.ContainerJSON) CreateContainerRequest {
	return ContainerToCreateRequestWithImageEnv(c, nil)
}

// ContainerToCreateRequestWithImageEnv extracts configuration parameters from an inspected container
// and filters out environment variables that are identical to base image defaults.
func ContainerToCreateRequestWithImageEnv(c types.ContainerJSON, imageEnv []string) CreateContainerRequest {
	req := CreateContainerRequest{
		Name: strings.TrimPrefix(c.Name, "/"),
	}

	if c.Config != nil {
		req.Image = c.Config.Image
		if len(c.Config.Env) > 0 {
			req.Env = FilterUserEnv(c.Config.Env, imageEnv)
		}
		if len(c.Config.Labels) > 0 {
			req.Labels = make(map[string]string, len(c.Config.Labels))
			for k, v := range c.Config.Labels {
				req.Labels[k] = v
			}
		}
	}
	if req.Image == "" && c.Image != "" {
		req.Image = c.Image
	}

	seenVolumes := make(map[string]bool)
	seenPorts := make(map[string]bool)

	if c.HostConfig != nil {
		req.RestartPolicy = string(c.HostConfig.RestartPolicy.Name)
		if c.HostConfig.Memory > 0 {
			req.MemoryMB = c.HostConfig.Memory / (1024 * 1024)
		}
		if c.HostConfig.NanoCPUs > 0 {
			req.CPUCores = float64(c.HostConfig.NanoCPUs) / 1e9
		}

		for _, b := range c.HostConfig.Binds {
			parts := strings.Split(b, ":")
			if len(parts) >= 2 {
				hostPath := parts[0]
				containerPath := parts[1]
				readOnly := false
				if len(parts) >= 3 && strings.Contains(parts[2], "ro") {
					readOnly = true
				}
				key := fmt.Sprintf("%s:%s", hostPath, containerPath)
				if !seenVolumes[key] {
					seenVolumes[key] = true
					req.Volumes = append(req.Volumes, VolumeMount{
						HostPath:      hostPath,
						ContainerPath: containerPath,
						ReadOnly:      readOnly,
					})
				}
			}
		}

		for port, bindings := range c.HostConfig.PortBindings {
			containerPort := port.Port()
			protocol := port.Proto()
			if len(bindings) > 0 {
				for _, b := range bindings {
					key := fmt.Sprintf("%s/%s->%s", containerPort, protocol, b.HostPort)
					if !seenPorts[key] {
						seenPorts[key] = true
						req.Ports = append(req.Ports, PortMapping{
							HostPort:      b.HostPort,
							ContainerPort: containerPort,
							Protocol:      protocol,
						})
					}
				}
			} else {
				key := fmt.Sprintf("%s/%s->", containerPort, protocol)
				if !seenPorts[key] {
					seenPorts[key] = true
					req.Ports = append(req.Ports, PortMapping{
						HostPort:      "",
						ContainerPort: containerPort,
						Protocol:      protocol,
					})
				}
			}
		}
	}

	for _, m := range c.Mounts {
		key := fmt.Sprintf("%s:%s", m.Source, m.Destination)
		if !seenVolumes[key] {
			seenVolumes[key] = true
			req.Volumes = append(req.Volumes, VolumeMount{
				HostPath:      m.Source,
				ContainerPath: m.Destination,
				ReadOnly:      !m.RW || strings.Contains(m.Mode, "ro"),
			})
		}
	}

	if c.NetworkSettings != nil && c.NetworkSettings.Ports != nil {
		for port, bindings := range c.NetworkSettings.Ports {
			containerPort := port.Port()
			protocol := port.Proto()
			if len(bindings) > 0 {
				for _, b := range bindings {
					key := fmt.Sprintf("%s/%s->%s", containerPort, protocol, b.HostPort)
					if !seenPorts[key] {
						seenPorts[key] = true
						req.Ports = append(req.Ports, PortMapping{
							HostPort:      b.HostPort,
							ContainerPort: containerPort,
							Protocol:      protocol,
						})
					}
				}
			} else {
				key := fmt.Sprintf("%s/%s->", containerPort, protocol)
				if !seenPorts[key] {
					seenPorts[key] = true
					req.Ports = append(req.Ports, PortMapping{
						HostPort:      "",
						ContainerPort: containerPort,
						Protocol:      protocol,
					})
				}
			}
		}
	}

	return req
}

func (s *Service) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	return s.cli.ContainerList(ctx, container.ListOptions{All: all})
}

func (s *Service) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	return s.cli.ContainerInspect(ctx, id)
}

func (s *Service) StartContainer(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *Service) StopContainer(ctx context.Context, id string, timeoutSeconds int) error {
	timeout := timeoutSeconds
	return s.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (s *Service) RestartContainer(ctx context.Context, id string, timeoutSeconds int) error {
	timeout := timeoutSeconds
	return s.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (s *Service) PauseContainer(ctx context.Context, id string) error {
	return s.cli.ContainerPause(ctx, id)
}

func (s *Service) UnpauseContainer(ctx context.Context, id string) error {
	return s.cli.ContainerUnpause(ctx, id)
}

func (s *Service) RemoveContainer(ctx context.Context, id string, force, removeVolumes bool) error {
	return s.cli.ContainerRemove(ctx, id, container.RemoveOptions{
		Force:         force,
		RemoveVolumes: removeVolumes,
	})
}

func (s *Service) KillContainer(ctx context.Context, id, signal string) error {
	if signal == "" {
		signal = "SIGKILL"
	}
	return s.cli.ContainerKill(ctx, id, signal)
}

func (s *Service) PruneContainers(ctx context.Context) (container.PruneReport, error) {
	return s.cli.ContainersPrune(ctx, filters.NewArgs())
}

func (s *Service) CreateContainer(ctx context.Context, req CreateContainerRequest) (string, error) {
	if err := req.Validate(); err != nil {
		return "", err
	}

	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, p := range req.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		port, err := nat.NewPort(proto, p.ContainerPort)
		if err != nil {
			return "", fmt.Errorf("invalid container port %s/%s: %w", p.ContainerPort, proto, err)
		}
		exposedPorts[port] = struct{}{}
		portBindings[port] = append(portBindings[port], nat.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: p.HostPort,
		})
	}

	binds := make([]string, 0, len(req.Volumes))
	for _, v := range req.Volumes {
		mount := fmt.Sprintf("%s:%s", v.HostPath, v.ContainerPath)
		if v.ReadOnly {
			mount += ":ro"
		}
		binds = append(binds, mount)
	}

	config := &container.Config{
		Image:        req.Image,
		Env:          req.Env,
		Labels:       req.Labels,
		ExposedPorts: exposedPorts,
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(req.RestartPolicy),
		},
	}

	if req.LogDriver != "" || len(req.LogOpts) > 0 {
		driver := req.LogDriver
		if driver == "" {
			driver = "json-file"
		}
		hostConfig.LogConfig = container.LogConfig{
			Type:   driver,
			Config: req.LogOpts,
		}
	}

	if req.MemoryMB > 0 {
		hostConfig.Resources.Memory = req.MemoryMB * 1024 * 1024
	}
	if req.CPUCores > 0 {
		hostConfig.Resources.NanoCPUs = int64(req.CPUCores * 1e9)
	}

	resp, err := s.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, req.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

func (s *Service) SystemInfo(ctx context.Context) (system.Info, error) {
	return s.cli.Info(ctx)
}
