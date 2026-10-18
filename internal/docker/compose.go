package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/go-connections/nat"
	"github.com/podiax/pdswarm/internal/store"
	"gopkg.in/yaml.v3"
)

// DeployEvent represents a progress event during stack deployment.
type DeployEvent struct {
	Step      string `json:"step"`               // "init", "network", "pull", "create", "start", "done", "error"
	Service   string `json:"service,omitempty"`
	Image     string `json:"image,omitempty"`
	Status    string `json:"status"`             // Human readable status description
	Progress  string `json:"progress,omitempty"` // e.g. "[=====>   ] 24.5MB/50MB"
	Detail    string `json:"detail,omitempty"`   // Extra info e.g. layer ID
	Timestamp int64  `json:"timestamp"`
}

// DeployProgressFunc is a callback invoked during stack deployment with progress events.
type DeployProgressFunc func(event DeployEvent)

const (
	ComposeProjectLabel = "com.docker.compose.project"
	ComposeServiceLabel = "com.docker.compose.service"
	ComposeVersionLabel = "com.docker.compose.version"
)

// ContainerSummary is an alias for types.Container.
type ContainerSummary = types.Container

// StackLabels generates the standard Docker Compose labels used by PDSwarm.
func StackLabels(projectName, serviceName string) map[string]string {
	return map[string]string{
		ComposeProjectLabel: projectName,
		ComposeServiceLabel: serviceName,
		ComposeVersionLabel: "pdswarm",
	}
}

// ComposeConfig represents the root structure of a Docker Compose YAML document.
type ComposeConfig struct {
	Version  string                           `yaml:"version,omitempty"`
	Services map[string]ComposeServiceConfig  `yaml:"services"`
	Networks map[string]ComposeNetworkConfig  `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}           `yaml:"volumes,omitempty"`
}

// ComposeServiceConfig represents a service definition in Docker Compose.
type ComposeServiceConfig struct {
	ContainerName string          `yaml:"container_name,omitempty"`
	Image         string          `yaml:"image"`
	Command       CommandList     `yaml:"command,omitempty"`
	Entrypoint    CommandList     `yaml:"entrypoint,omitempty"`
	Environment   EnvironmentList `yaml:"environment,omitempty"`
	Ports         PortList        `yaml:"ports,omitempty"`
	Volumes       VolumeList      `yaml:"volumes,omitempty"`
	Restart       string          `yaml:"restart,omitempty"`
	Networks      NetworkList     `yaml:"networks,omitempty"`
	Labels        LabelMap        `yaml:"labels,omitempty"`
}

// ComposeNetworkConfig represents a network definition in Docker Compose.
type ComposeNetworkConfig struct {
	Driver     string            `yaml:"driver,omitempty"`
	Attachable bool              `yaml:"attachable,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

// CommandList unmarshals command or entrypoint from a string or a sequence of strings.
type CommandList []string

func (c *CommandList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*c = parseCommandString(value.Value)
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*c = list
		return nil
	default:
		return fmt.Errorf("unexpected yaml kind for command: %v", value.Kind)
	}
}

// EnvironmentList unmarshals environment from a list ("K=V") or map (k: v).
type EnvironmentList []string

func (e *EnvironmentList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*e = list
		return nil
	case yaml.MappingNode:
		var list []string
		for i := 0; i < len(value.Content); i += 2 {
			k := value.Content[i].Value
			v := value.Content[i+1].Value
			list = append(list, fmt.Sprintf("%s=%s", k, v))
		}
		*e = list
		return nil
	default:
		return fmt.Errorf("unexpected yaml kind for environment: %v", value.Kind)
	}
}

// PortList unmarshals ports from either string mappings or integer ports.
type PortList []PortMapping

func (p *PortList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("expected sequence for ports, got kind %v", value.Kind)
	}
	var ports []PortMapping
	for _, node := range value.Content {
		switch node.Kind {
		case yaml.ScalarNode:
			pm, err := parsePortString(node.Value)
			if err != nil {
				return err
			}
			ports = append(ports, pm)
		default:
			return fmt.Errorf("unexpected yaml kind for port item: %v", node.Kind)
		}
	}
	*p = ports
	return nil
}

// VolumeList unmarshals volumes from string declarations.
type VolumeList []VolumeMount

func (v *VolumeList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("expected sequence for volumes, got kind %v", value.Kind)
	}
	var volumes []VolumeMount
	for _, node := range value.Content {
		if node.Kind == yaml.ScalarNode {
			volumes = append(volumes, parseVolumeString(node.Value))
		}
	}
	*v = volumes
	return nil
}

// NetworkList unmarshals networks from a sequence or map.
type NetworkList []string

func (n *NetworkList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*n = list
		return nil
	case yaml.MappingNode:
		var list []string
		for i := 0; i < len(value.Content); i += 2 {
			list = append(list, value.Content[i].Value)
		}
		*n = list
		return nil
	default:
		return fmt.Errorf("unexpected yaml kind for networks: %v", value.Kind)
	}
}

// LabelMap unmarshals labels from a mapping or sequence of "k=v" strings.
type LabelMap map[string]string

func (l *LabelMap) UnmarshalYAML(value *yaml.Node) error {
	m := make(map[string]string)
	switch value.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			m[value.Content[i].Value] = value.Content[i+1].Value
		}
		*l = m
		return nil
	case yaml.SequenceNode:
		for _, node := range value.Content {
			parts := strings.SplitN(node.Value, "=", 2)
			if len(parts) == 2 {
				m[parts[0]] = parts[1]
			} else {
				m[parts[0]] = ""
			}
		}
		*l = m
		return nil
	default:
		return fmt.Errorf("unexpected yaml kind for labels: %v", value.Kind)
	}
}

// ParseComposeYAML parses and validates a Compose YAML string.
func ParseComposeYAML(content string) (*ComposeConfig, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, errors.New("compose YAML content cannot be empty")
	}

	var cfg ComposeConfig
	if err := yaml.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	if len(cfg.Services) == 0 {
		return nil, errors.New("no services defined in compose file")
	}

	for name, svc := range cfg.Services {
		if strings.TrimSpace(svc.Image) == "" {
			return nil, fmt.Errorf("service %q is missing an image", name)
		}
	}

	return &cfg, nil
}

// DeployComposeStack deploys a stack by name and compose YAML content.
func (s *Service) DeployComposeStack(ctx context.Context, name, composeYAML string) error {
	return s.DeployComposeStackWithProgress(ctx, name, composeYAML, nil)
}

// DeployComposeStackWithProgress deploys a stack with real-time progress callbacks.
func (s *Service) DeployComposeStackWithProgress(ctx context.Context, name, composeYAML string, onProgress DeployProgressFunc) error {
	return s.DeployStackWithProgress(ctx, &store.Stack{
		Name:        name,
		ComposeYAML: composeYAML,
	}, onProgress)
}

// DeployStack deploys a stack defined by store.Stack into the Docker daemon.
func (s *Service) DeployStack(ctx context.Context, stack *store.Stack) error {
	return s.DeployStackWithProgress(ctx, stack, nil)
}

// DeployStackWithProgress deploys a stack defined by store.Stack with real-time progress callbacks.
func (s *Service) DeployStackWithProgress(ctx context.Context, stack *store.Stack, onProgress DeployProgressFunc) error {
	if stack == nil {
		return errors.New("stack cannot be nil")
	}
	if strings.TrimSpace(stack.Name) == "" {
		return errors.New("stack name cannot be empty")
	}
	if strings.TrimSpace(stack.ComposeYAML) == "" {
		return errors.New("compose yaml cannot be empty")
	}

	emit := func(evt DeployEvent) {
		if onProgress != nil {
			if evt.Timestamp == 0 {
				evt.Timestamp = time.Now().Unix()
			}
			onProgress(evt)
		}
	}

	emit(DeployEvent{
		Step:   "init",
		Status: fmt.Sprintf("Preparing deployment for stack %q...", stack.Name),
	})

	cfg, err := ParseComposeYAML(stack.ComposeYAML)
	if err != nil {
		return fmt.Errorf("invalid compose yaml: %w", err)
	}

	// Auto-infer working directory from config file or container labels if not explicitly provided
	if stack.WorkingDir == "" && stack.ConfigFile != "" {
		stack.WorkingDir = filepath.Dir(stack.ConfigFile)
	}
	if stack.WorkingDir == "" {
		if exConts, cErr := s.GetStackContainers(ctx, stack.Name); cErr == nil && len(exConts) > 0 {
			for _, ec := range exConts {
				if wDir := ec.Labels["com.docker.compose.project.working_dir"]; wDir != "" {
					stack.WorkingDir = wDir
					break
				}
				if cfgFile := ec.Labels["com.docker.compose.project.config_files"]; cfgFile != "" {
					files := strings.Split(cfgFile, ",")
					if len(files) > 0 && strings.TrimSpace(files[0]) != "" {
						stack.WorkingDir = filepath.Dir(strings.TrimSpace(files[0]))
						break
					}
				}
			}
		}
	}

	// Determine and ensure project networks
	defaultNetName := fmt.Sprintf("%s_default", stack.Name)
	projectNets := make(map[string]string) // network name -> driver
	projectNets[defaultNetName] = "bridge"

	if len(cfg.Networks) > 0 {
		for netName, netCfg := range cfg.Networks {
			driver := netCfg.Driver
			if driver == "" {
				driver = "bridge"
			}
			projectNets[fmt.Sprintf("%s_%s", stack.Name, netName)] = driver
		}
	}

	existingNets, err := s.cli.NetworkList(ctx, network.ListOptions{})
	existingMap := make(map[string]string)
	if err == nil {
		for _, n := range existingNets {
			existingMap[n.Name] = n.ID
		}
	}

	for netName, driver := range projectNets {
		if _, exists := existingMap[netName]; !exists {
			emit(DeployEvent{
				Step:   "network",
				Status: fmt.Sprintf("Creating network %s...", netName),
			})
			_, _ = s.cli.NetworkCreate(ctx, netName, network.CreateOptions{
				Driver: driver,
				Labels: map[string]string{
					ComposeProjectLabel: stack.Name,
					ComposeVersionLabel: "pdswarm",
				},
			})
		}
	}

	// Ensure top-level named volumes are created with project prefix if declared in volumes section
	if cfg.Volumes != nil {
		for volName := range cfg.Volumes {
			fullVolName := fmt.Sprintf("%s_%s", stack.Name, volName)
			_, _ = s.cli.VolumeCreate(ctx, volume.CreateOptions{
				Name: fullVolName,
				Labels: map[string]string{
					ComposeProjectLabel: stack.Name,
					ComposeVersionLabel: "pdswarm",
				},
			})
		}
	}

	// Deploy each service
	for serviceName, svcCfg := range cfg.Services {
		// Pull image if not already present
		_, _, inspectErr := s.cli.ImageInspectWithRaw(ctx, svcCfg.Image)
		if inspectErr != nil {
			emit(DeployEvent{
				Step:    "pull",
				Service: serviceName,
				Image:   svcCfg.Image,
				Status:  fmt.Sprintf("Pulling image %s...", svcCfg.Image),
			})

			reader, pullErr := s.cli.ImagePull(ctx, svcCfg.Image, image.PullOptions{})
			if pullErr != nil {
				emit(DeployEvent{
					Step:    "error",
					Service: serviceName,
					Image:   svcCfg.Image,
					Status:  fmt.Sprintf("Failed to pull image %s", svcCfg.Image),
					Detail:  pullErr.Error(),
				})
				return fmt.Errorf("failed to pull image %s: %w", svcCfg.Image, pullErr)
			}
			if reader != nil {
				if onProgress != nil {
					scanner := bufio.NewScanner(reader)
					lastEmit := time.Now()
					for scanner.Scan() {
						line := scanner.Bytes()
						if len(line) == 0 {
							continue
						}
						var dMsg struct {
							Status   string `json:"status"`
							Progress string `json:"progress"`
							ID       string `json:"id"`
							Error    string `json:"error"`
						}
						if err := json.Unmarshal(line, &dMsg); err == nil {
							if dMsg.Error != "" {
								_ = reader.Close()
								return fmt.Errorf("docker pull error: %s", dMsg.Error)
							}
							now := time.Now()
							if dMsg.Progress == "" || now.Sub(lastEmit) >= 150*time.Millisecond {
								lastEmit = now
								emit(DeployEvent{
									Step:     "pull",
									Service:  serviceName,
									Image:    svcCfg.Image,
									Status:   dMsg.Status,
									Progress: dMsg.Progress,
									Detail:   dMsg.ID,
								})
							}
						}
					}
				} else {
					_, _ = io.Copy(io.Discard, reader)
				}
				_ = reader.Close()
			}
			emit(DeployEvent{
				Step:    "pull",
				Service: serviceName,
				Image:   svcCfg.Image,
				Status:  fmt.Sprintf("Image %s ready", svcCfg.Image),
			})
		} else {
			emit(DeployEvent{
				Step:    "pull",
				Service: serviceName,
				Image:   svcCfg.Image,
				Status:  fmt.Sprintf("Image %s already present locally", svcCfg.Image),
			})
		}

		containerName := fmt.Sprintf("%s-%s-1", stack.Name, serviceName)
		if svcCfg.ContainerName != "" {
			containerName = svcCfg.ContainerName
		}

		// Check if any container already exists for this service in this stack, stop and remove it
		existingContainers, _ := s.GetStackContainers(ctx, stack.Name)
		for _, ec := range existingContainers {
			if ec.Labels[ComposeServiceLabel] == serviceName || (len(ec.Names) > 0 && strings.TrimPrefix(ec.Names[0], "/") == containerName) {
				timeout := 5
				_ = s.cli.ContainerStop(ctx, ec.ID, container.StopOptions{Timeout: &timeout})
				_ = s.cli.ContainerRemove(ctx, ec.ID, container.RemoveOptions{Force: true, RemoveVolumes: false})
			}
		}

		// Also check by direct container name inspect
		_, inspectContErr := s.cli.ContainerInspect(ctx, containerName)
		if inspectContErr == nil {
			timeout := 5
			_ = s.cli.ContainerStop(ctx, containerName, container.StopOptions{Timeout: &timeout})
			_ = s.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true, RemoveVolumes: false})
		}

		// Prepare ports
		exposedPorts := nat.PortSet{}
		portBindings := nat.PortMap{}
		for _, p := range svcCfg.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			port, err := nat.NewPort(proto, p.ContainerPort)
			if err != nil {
				return fmt.Errorf("invalid port %s/%s for service %s: %w", p.ContainerPort, proto, serviceName, err)
			}
			exposedPorts[port] = struct{}{}
			if p.HostPort != "" {
				portBindings[port] = append(portBindings[port], nat.PortBinding{
					HostIP:   "0.0.0.0",
					HostPort: p.HostPort,
				})
			}
		}

		// Prepare volume binds and anonymous volumes
		volumes := make(map[string]struct{})
		var binds []string
		for _, v := range svcCfg.Volumes {
			if v.HostPath == "" {
				volumes[v.ContainerPath] = struct{}{}
			} else {
				hostPath := v.HostPath
				// Check if hostPath is a named volume defined in top-level volumes:
				if cfg.Volumes != nil {
					if _, isNamed := cfg.Volumes[hostPath]; isNamed {
						prefix := fmt.Sprintf("%s_", stack.Name)
						if !strings.HasPrefix(hostPath, prefix) {
							hostPath = prefix + hostPath
						}
					}
				}

				// If it's a relative host path (e.g. ./data or ../data or .):
				if strings.HasPrefix(hostPath, "./") || strings.HasPrefix(hostPath, "../") || hostPath == "." {
					if stack.WorkingDir != "" {
						hostPath = filepath.Clean(filepath.Join(stack.WorkingDir, hostPath))
					} else {
						// Fallback: match destination against existing container mounts
						for _, ec := range existingContainers {
							if inspect, err := s.cli.ContainerInspect(ctx, ec.ID); err == nil {
								for _, m := range inspect.Mounts {
									if m.Destination == v.ContainerPath && m.Source != "" {
										hostPath = m.Source
										break
									}
								}
							}
							if !strings.HasPrefix(hostPath, ".") {
								break
							}
						}
					}
				}

				b := fmt.Sprintf("%s:%s", hostPath, v.ContainerPath)
				if v.ReadOnly {
					b += ":ro"
				}
				binds = append(binds, b)
			}
		}

		// Prepare labels
		labels := StackLabels(stack.Name, serviceName)
		for k, v := range svcCfg.Labels {
			labels[k] = v
		}

		targetNet := defaultNetName
		if len(svcCfg.Networks) > 0 {
			targetNet = fmt.Sprintf("%s_%s", stack.Name, svcCfg.Networks[0])
		}

		netConfig := &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				targetNet: {
					Aliases: []string{serviceName},
				},
			},
		}

		containerConfig := &container.Config{
			Image:        svcCfg.Image,
			Cmd:          strslice.StrSlice(svcCfg.Command),
			Entrypoint:   strslice.StrSlice(svcCfg.Entrypoint),
			Env:          svcCfg.Environment,
			Labels:       labels,
			ExposedPorts: exposedPorts,
			Volumes:      volumes,
		}

		hostConfig := &container.HostConfig{
			PortBindings: portBindings,
			Binds:        binds,
			RestartPolicy: container.RestartPolicy{
				Name: container.RestartPolicyMode(svcCfg.Restart),
			},
		}

		emit(DeployEvent{
			Step:    "create",
			Service: serviceName,
			Status:  fmt.Sprintf("Creating container %s...", containerName),
		})

		resp, err := s.cli.ContainerCreate(ctx, containerConfig, hostConfig, netConfig, nil, containerName)
		if err != nil {
			return fmt.Errorf("failed to create container %s: %w", containerName, err)
		}

		// Connect any additional networks
		if len(svcCfg.Networks) > 1 {
			for _, extraNet := range svcCfg.Networks[1:] {
				extraNetName := fmt.Sprintf("%s_%s", stack.Name, extraNet)
				_ = s.cli.NetworkConnect(ctx, extraNetName, resp.ID, &network.EndpointSettings{
					Aliases: []string{serviceName},
				})
			}
		}

		emit(DeployEvent{
			Step:    "start",
			Service: serviceName,
			Status:  fmt.Sprintf("Starting container %s...", containerName),
		})

		if err := s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return fmt.Errorf("failed to start container %s: %w", containerName, err)
		}
	}

	emit(DeployEvent{
		Step:   "done",
		Status: fmt.Sprintf("Stack %s successfully deployed and running!", stack.Name),
	})

	stack.Status = "running"
	stack.UpdatedAt = time.Now()
	return nil
}

// StopStack finds all containers belonging to the stack and stops them.
func (s *Service) StopStack(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("stack name cannot be empty")
	}

	containers, err := s.GetStackContainers(ctx, name)
	if err != nil {
		return err
	}

	timeout := 10
	var errs []error
	for _, c := range containers {
		if err := s.cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop one or more containers: %v", errs)
	}

	return nil
}

// DownStack stops and removes all containers and project networks associated with the stack.
func (s *Service) DownStack(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("stack name cannot be empty")
	}

	containers, err := s.GetStackContainers(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get stack containers: %w", err)
	}

	var errs []error
	timeout := 5
	for _, c := range containers {
		_ = s.cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
		if rmErr := s.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true, RemoveVolumes: false}); rmErr != nil {
			errs = append(errs, rmErr)
		}
	}

	filterArgs := filters.NewArgs(filters.Arg("label", fmt.Sprintf("%s=%s", ComposeProjectLabel, name)))
	nets, err := s.cli.NetworkList(ctx, network.ListOptions{Filters: filterArgs})
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, n := range nets {
			if rmErr := s.cli.NetworkRemove(ctx, n.ID); rmErr != nil {
				errs = append(errs, rmErr)
			}
		}
	}

	defaultNetName := fmt.Sprintf("%s_default", name)
	allNets, err := s.cli.NetworkList(ctx, network.ListOptions{})
	if err == nil {
		for _, n := range allNets {
			if n.Name == defaultNetName {
				if rmErr := s.cli.NetworkRemove(ctx, n.ID); rmErr != nil {
					errs = append(errs, rmErr)
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during down stack: %v", errs)
	}

	return nil
}

// GetStackContainers returns all containers labeled with the stack's project name.
func (s *Service) GetStackContainers(ctx context.Context, name string) ([]types.Container, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("stack name cannot be empty")
	}

	filterArgs := filters.NewArgs(filters.Arg("label", fmt.Sprintf("%s=%s", ComposeProjectLabel, name)))
	return s.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
}

var envPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?::?[-?]([^}]*))?\}`)

func resolveEnvPortDefault(val string) string {
	val = strings.Trim(val, "\"'")
	m := envPattern.FindStringSubmatch(val)
	if len(m) > 0 {
		if len(m) > 2 && m[2] != "" {
			return m[2]
		}
		return ""
	}
	return val
}

func splitPortString(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '$' && i+1 < len(s) && s[i+1] == '{' {
			depth++
			current.WriteByte(ch)
			current.WriteByte('{')
			i++
			continue
		}
		if ch == '{' && depth > 0 {
			depth++
			current.WriteByte(ch)
			continue
		}
		if ch == '}' && depth > 0 {
			depth--
			current.WriteByte(ch)
			continue
		}
		if ch == ':' && depth == 0 {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	parts = append(parts, current.String())
	return parts
}

func parsePortString(s string) (PortMapping, error) {
	proto := "tcp"
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		proto = strings.ToLower(s[idx+1:])
		s = s[:idx]
	}

	parts := splitPortString(s)
	switch len(parts) {
	case 1:
		raw := strings.TrimSpace(parts[0])
		portNum := resolveEnvPortDefault(raw)
		if portNum == "" {
			portNum = raw
		}
		if _, err := strconv.Atoi(portNum); err != nil {
			return PortMapping{}, fmt.Errorf("invalid container port: %s", raw)
		}
		return PortMapping{HostPort: portNum, ContainerPort: portNum, Protocol: proto}, nil
	case 2:
		hp := resolveEnvPortDefault(strings.TrimSpace(parts[0]))
		cp := resolveEnvPortDefault(strings.TrimSpace(parts[1]))
		return PortMapping{HostPort: hp, ContainerPort: cp, Protocol: proto}, nil
	case 3:
		hp := resolveEnvPortDefault(strings.TrimSpace(parts[1]))
		cp := resolveEnvPortDefault(strings.TrimSpace(parts[2]))
		return PortMapping{HostPort: hp, ContainerPort: cp, Protocol: proto}, nil
	default:
		return PortMapping{}, fmt.Errorf("invalid port format: %s", s)
	}
}

func parseVolumeString(s string) VolumeMount {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 1:
		return VolumeMount{ContainerPath: parts[0]}
	case 2:
		return VolumeMount{HostPath: parts[0], ContainerPath: parts[1]}
	case 3:
		ro := strings.Contains(parts[2], "ro")
		return VolumeMount{HostPath: parts[0], ContainerPath: parts[1], ReadOnly: ro}
	default:
		return VolumeMount{HostPath: parts[0], ContainerPath: parts[1]}
	}
}

func parseCommandString(cmdStr string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	var quoteChar rune

	for _, r := range cmdStr {
		switch {
		case r == '\'' || r == '"':
			if inQuote && r == quoteChar {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			} else {
				current.WriteRune(r)
			}
		case unicode.IsSpace(r) && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// RecoverComposeFromPortainer scans running or stopped Portainer containers on the Docker daemon
// and attempts to read the specified compose file (e.g. /data/compose/70/v7/docker-compose.yml).
func (s *Service) RecoverComposeFromPortainer(ctx context.Context, configFile string) (string, error) {
	if s == nil || s.cli == nil {
		return "", errors.New("docker service not available")
	}

	allContainers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	var portainerContainers []types.Container
	for _, c := range allContainers {
		isPortainer := strings.Contains(strings.ToLower(c.Image), "portainer")
		if !isPortainer {
			for _, n := range c.Names {
				if strings.Contains(strings.ToLower(n), "portainer") {
					isPortainer = true
					break
				}
			}
		}
		if isPortainer {
			portainerContainers = append(portainerContainers, c)
		}
	}

	if len(portainerContainers) == 0 {
		return "", errors.New("no portainer container found")
	}

	for _, pc := range portainerContainers {
		// 1. Try reading directly from the container via Docker API
		pathsToTry := []string{configFile}
		if strings.HasPrefix(configFile, "/data/") {
			pathsToTry = append(pathsToTry, strings.TrimPrefix(configFile, "/data"))
		} else if !strings.HasPrefix(configFile, "/") {
			pathsToTry = append(pathsToTry, "/"+configFile)
			pathsToTry = append(pathsToTry, "/data/"+configFile)
		} else {
			pathsToTry = append(pathsToTry, "/data"+configFile)
		}

		for _, p := range pathsToTry {
			data, _, err := s.ReadContainerFile(ctx, pc.ID, p, 5*1024*1024)
			if err == nil && len(data) > 0 {
				return string(data), nil
			}
		}

		// 2. Check Portainer container volume mounts on the host filesystem
		cj, err := s.InspectContainer(ctx, pc.ID)
		if err == nil {
			for _, m := range cj.Mounts {
				if m.Destination == "/data" && m.Source != "" {
					rel := strings.TrimPrefix(configFile, "/data")
					candidate := filepath.Join(m.Source, rel)
					if hostData, err := os.ReadFile(candidate); err == nil && len(hostData) > 0 {
						return string(hostData), nil
					}
				}
				if m.Type == "volume" && m.Name != "" {
					rel := strings.TrimPrefix(configFile, "/data")
					candidate := filepath.Join("/var/lib/docker/volumes", m.Name, "_data", rel)
					if hostData, err := os.ReadFile(candidate); err == nil && len(hostData) > 0 {
						return string(hostData), nil
					}
				}
			}
		}
	}

	return "", errors.New("compose file not found inside portainer container")
}

// SynthesizeComposeYAML inspects the containers belonging to a stack and generates
// a complete, standard Docker Compose YAML definition.
func SynthesizeComposeYAML(ctx context.Context, ds *Service, stackName string, containers []ContainerSummary) (string, error) {
	if len(containers) == 0 {
		return "", errors.New("no containers provided to synthesize compose yaml")
	}

	services := make(map[string]map[string]interface{})
	topVolumes := make(map[string]interface{})
	topNetworks := make(map[string]interface{})
	var serviceNames []string

	for _, c := range containers {
		var cj types.ContainerJSON
		var err error
		if ds != nil {
			cj, err = ds.InspectContainer(ctx, c.ID)
		}
		if err != nil || ds == nil {
			cj = types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID:    c.ID,
					Name:  "",
					State: &types.ContainerState{Running: strings.ToLower(c.State) == "running"},
				},
				Config: &container.Config{
					Image:  c.Image,
					Labels: c.Labels,
				},
				Mounts: c.Mounts,
			}
			if len(c.Names) > 0 {
				cj.Name = c.Names[0]
			}
		}

		svcName := c.Labels[ComposeServiceLabel]
		if svcName == "" {
			svcName = strings.TrimPrefix(cj.Name, "/")
			if svcName == "" {
				subId := c.ID
				if len(subId) > 12 {
					subId = subId[:12]
				}
				svcName = fmt.Sprintf("service-%s", subId)
			}
		}

		svcName = strings.TrimSpace(svcName)
		if _, exists := services[svcName]; !exists {
			serviceNames = append(serviceNames, svcName)
		}

		svc := make(map[string]interface{})

		// Image
		targetImage := ""
		if cj.Config != nil && cj.Config.Image != "" {
			targetImage = cj.Config.Image
		} else if c.Image != "" {
			targetImage = c.Image
		}
		svc["image"] = targetImage

		// Container Name
		cleanName := strings.TrimPrefix(cj.Name, "/")
		if cleanName != "" && cleanName != svcName {
			svc["container_name"] = cleanName
		}

		// Restart Policy
		if cj.HostConfig != nil && cj.HostConfig.RestartPolicy.Name != "" && cj.HostConfig.RestartPolicy.Name != "no" {
			svc["restart"] = string(cj.HostConfig.RestartPolicy.Name)
		}

		// Ports
		var portStrs []string
		seenPorts := make(map[string]bool)
		if cj.HostConfig != nil && cj.HostConfig.PortBindings != nil {
			for port, bindings := range cj.HostConfig.PortBindings {
				for _, b := range bindings {
					pStr := fmt.Sprintf("%s:%s", b.HostPort, port.Port())
					if port.Proto() != "" && port.Proto() != "tcp" {
						pStr += "/" + port.Proto()
					}
					if !seenPorts[pStr] && b.HostPort != "" {
						seenPorts[pStr] = true
						portStrs = append(portStrs, pStr)
					}
				}
			}
		} else if len(c.Ports) > 0 {
			for _, p := range c.Ports {
				if p.PublicPort > 0 {
					pStr := fmt.Sprintf("%d:%d", p.PublicPort, p.PrivatePort)
					if p.Type != "" && p.Type != "tcp" {
						pStr += "/" + p.Type
					}
					if !seenPorts[pStr] {
						seenPorts[pStr] = true
						portStrs = append(portStrs, pStr)
					}
				}
			}
		}
		if len(portStrs) > 0 {
			sort.Strings(portStrs)
			svc["ports"] = portStrs
		}

		// Environment
		if cj.Config != nil && len(cj.Config.Env) > 0 {
			filtered := filterSystemEnv(cj.Config.Env)
			if len(filtered) > 0 {
				svc["environment"] = filtered
			}
		}

		// Volumes
		var volStrs []string
		seenVols := make(map[string]bool)
		for _, m := range cj.Mounts {
			var volStr string
			if m.Type == "bind" {
				volStr = fmt.Sprintf("%s:%s", m.Source, m.Destination)
				if !m.RW || strings.Contains(m.Mode, "ro") {
					volStr += ":ro"
				}
			} else if m.Type == "volume" {
				volStr = fmt.Sprintf("%s:%s", m.Name, m.Destination)
				if !m.RW || strings.Contains(m.Mode, "ro") {
					volStr += ":ro"
				}
				topVolumes[m.Name] = map[string]interface{}{}
			}
			if volStr != "" && !seenVols[volStr] {
				seenVols[volStr] = true
				volStrs = append(volStrs, volStr)
			}
		}
		if len(volStrs) > 0 {
			sort.Strings(volStrs)
			svc["volumes"] = volStrs
		}

		// Networks
		if cj.NetworkSettings != nil && cj.NetworkSettings.Networks != nil {
			var netStrs []string
			for netName := range cj.NetworkSettings.Networks {
				if netName != "bridge" && netName != "host" && netName != "none" {
					netStrs = append(netStrs, netName)
					topNetworks[netName] = map[string]interface{}{}
				}
			}
			if len(netStrs) > 0 {
				sort.Strings(netStrs)
				svc["networks"] = netStrs
			}
		}

		// Command
		if cj.Config != nil && len(cj.Config.Cmd) > 0 {
			svc["command"] = []string(cj.Config.Cmd)
		}

		services[svcName] = svc
	}

	var sb strings.Builder
	sb.WriteString("# Reconstructed by PDSwarm from live containers\n")
	sb.WriteString("services:\n")

	for _, svcName := range serviceNames {
		svc := services[svcName]
		sb.WriteString(fmt.Sprintf("  %s:\n", svcName))

		if img, ok := svc["image"].(string); ok && img != "" {
			sb.WriteString(fmt.Sprintf("    image: %s\n", img))
		}
		if cName, ok := svc["container_name"].(string); ok && cName != "" {
			sb.WriteString(fmt.Sprintf("    container_name: %s\n", cName))
		}
		if restart, ok := svc["restart"].(string); ok && restart != "" {
			sb.WriteString(fmt.Sprintf("    restart: %s\n", restart))
		}
		if ports, ok := svc["ports"].([]string); ok && len(ports) > 0 {
			sb.WriteString("    ports:\n")
			for _, p := range ports {
				sb.WriteString(fmt.Sprintf("      - %q\n", p))
			}
		}
		if env, ok := svc["environment"].([]string); ok && len(env) > 0 {
			sb.WriteString("    environment:\n")
			for _, e := range env {
				sb.WriteString(fmt.Sprintf("      - %q\n", e))
			}
		}
		if vols, ok := svc["volumes"].([]string); ok && len(vols) > 0 {
			sb.WriteString("    volumes:\n")
			for _, v := range vols {
				sb.WriteString(fmt.Sprintf("      - %q\n", v))
			}
		}
		if nets, ok := svc["networks"].([]string); ok && len(nets) > 0 {
			sb.WriteString("    networks:\n")
			for _, n := range nets {
				sb.WriteString(fmt.Sprintf("      - %s\n", n))
			}
		}
		if cmd, ok := svc["command"].([]string); ok && len(cmd) > 0 {
			cmdJSON, _ := json.Marshal(cmd)
			sb.WriteString(fmt.Sprintf("    command: %s\n", string(cmdJSON)))
		}
		sb.WriteString("\n")
	}

	if len(topVolumes) > 0 {
		sb.WriteString("volumes:\n")
		var volNames []string
		for v := range topVolumes {
			volNames = append(volNames, v)
		}
		sort.Strings(volNames)
		for _, v := range volNames {
			sb.WriteString(fmt.Sprintf("  %s:\n    external: true\n", v))
		}
		sb.WriteString("\n")
	}

	if len(topNetworks) > 0 {
		sb.WriteString("networks:\n")
		var netNames []string
		for n := range topNetworks {
			netNames = append(netNames, n)
		}
		sort.Strings(netNames)
		for _, n := range netNames {
			sb.WriteString(fmt.Sprintf("  %s:\n    external: true\n", n))
		}
	}

	return strings.TrimSpace(sb.String()) + "\n", nil
}

func filterSystemEnv(env []string) []string {
	var filtered []string
	systemPrefixes := []string{
		"PATH=",
		"HOSTNAME=",
		"HOME=",
		"GOSU_VERSION=",
		"GPG_KEY=",
		"LANG=",
		"LC_ALL=",
		"TERM=",
		"PWD=",
		"SHLVL=",
	}

	for _, e := range env {
		isSystem := false
		for _, prefix := range systemPrefixes {
			if strings.HasPrefix(e, prefix) {
				isSystem = true
				break
			}
		}
		if !isSystem {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// ExtractOrSynthesizeComposeYAML attempts to extract the Compose YAML for an external stack.
// It tries reading from local disk, recovering from a Portainer container, or synthesizing
// standard Compose YAML from the running containers.
func (s *Service) ExtractOrSynthesizeComposeYAML(ctx context.Context, stackName string, containers []ContainerSummary) (yamlContent string, configFilePath string, workingDir string) {
	if len(containers) == 0 {
		return "", "", ""
	}

	c := containers[0]
	workingDir = c.Labels["com.docker.compose.project.working_dir"]
	configFile := c.Labels["com.docker.compose.project.config_files"]

	if configFile != "" {
		files := strings.Split(configFile, ",")
		for _, f := range files {
			trimmed := strings.TrimSpace(f)
			if trimmed == "" {
				continue
			}
			configFilePath = trimmed

			// 1. Check if the file exists on the host filesystem directly
			if data, err := os.ReadFile(trimmed); err == nil && len(data) > 0 {
				yamlContent = string(data)
				if workingDir == "" {
					workingDir = filepath.Dir(trimmed)
				}
				return yamlContent, configFilePath, workingDir
			}

			// 2. If path is a Portainer compose path (e.g. /data/compose/...), try recovering from Portainer container
			if s != nil {
				if portainerYAML, err := s.RecoverComposeFromPortainer(ctx, trimmed); err == nil && len(portainerYAML) > 0 {
					yamlContent = portainerYAML
					if workingDir == "" {
						workingDir = filepath.Dir(trimmed)
					}
					return yamlContent, configFilePath, workingDir
				}
			}
		}
	}

	// 3. Fallback: Synthesize complete Compose YAML from inspected containers
	if s != nil && len(containers) > 0 {
		if synthesized, err := SynthesizeComposeYAML(ctx, s, stackName, containers); err == nil && len(synthesized) > 0 {
			yamlContent = synthesized
			if workingDir == "" && configFilePath != "" {
				workingDir = filepath.Dir(configFilePath)
			}
			return yamlContent, configFilePath, workingDir
		}
	}

	return "", configFilePath, workingDir
}

