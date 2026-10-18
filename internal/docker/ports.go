package docker

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// PortAllocation represents a host port bound by a Docker container or service.
type PortAllocation struct {
	HostIP        string `json:"host_ip"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // "tcp" or "udp"
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	State         string `json:"state"` // "running", "created", etc.
}

// PortConflict represents conflict analysis for a requested port mapping.
type PortConflict struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	InUse         bool   `json:"in_use"`
	ConflictWith  string `json:"conflict_with,omitempty"`
	SuggestedPort int    `json:"suggested_port,omitempty"`
}

// CheckPortsResult contains the full conflict analysis across multiple requested ports.
type CheckPortsResult struct {
	HasConflict bool           `json:"has_conflict"`
	Conflicts   []PortConflict `json:"conflicts"`
}

// GetAllocatedPorts returns all ports currently bound by containers on the Docker host.
func (s *Service) GetAllocatedPorts(ctx context.Context) ([]PortAllocation, error) {
	if s == nil || s.cli == nil {
		return nil, fmt.Errorf("docker service not available")
	}

	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var allocations []PortAllocation
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		} else if len(c.ID) >= 12 {
			name = c.ID[:12]
		}

		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				proto := strings.ToLower(p.Type)
				if proto == "" {
					proto = "tcp"
				}
				ip := p.IP
				if ip == "" {
					ip = "0.0.0.0"
				}

				allocations = append(allocations, PortAllocation{
					HostIP:        ip,
					HostPort:      int(p.PublicPort),
					ContainerPort: int(p.PrivatePort),
					Protocol:      proto,
					ContainerID:   c.ID,
					ContainerName: name,
					State:         c.State,
				})
			}
		}
	}

	return allocations, nil
}

// IsPortInUse checks whether a given host port and protocol are in use by either
// an existing Docker container or another host OS process.
func (s *Service) IsPortInUse(hostPort int, proto string, allocations []PortAllocation) (bool, string) {
	if hostPort <= 0 || hostPort > 65535 {
		return true, "invalid port number"
	}

	proto = strings.ToLower(proto)
	if proto == "" {
		proto = "tcp"
	}

	// 1. Check against known Docker container allocations
	for _, a := range allocations {
		if a.HostPort == hostPort && (a.Protocol == proto || proto == "all") {
			status := a.State
			if status == "" {
				status = "allocated"
			}
			return true, fmt.Sprintf("container %s (%s)", a.ContainerName, status)
		}
	}

	// 2. Check directly against host network stack
	if proto == "udp" {
		conn, err := net.ListenPacket("udp", fmt.Sprintf("0.0.0.0:%d", hostPort))
		if err != nil {
			return true, "host system process (UDP in use)"
		}
		_ = conn.Close()
	} else {
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", hostPort))
		if err != nil {
			return true, "host system process (TCP in use)"
		}
		_ = ln.Close()
	}

	return false, ""
}

// FindNextAvailablePort finds the nearest available port starting from startPort.
func (s *Service) FindNextAvailablePort(startPort int, proto string, allocations []PortAllocation) int {
	if startPort <= 0 {
		startPort = 1024
	}

	candidate := startPort
	for candidate <= 65535 {
		inUse, _ := s.IsPortInUse(candidate, proto, allocations)
		if !inUse {
			return candidate
		}
		candidate++
	}

	return startPort
}

// CheckPortConflicts inspects a list of port mapping strings and returns conflict details.
// Each port string can be "HOST:CONTAINER", "HOST:CONTAINER/PROTO", or simply "CONTAINER".
func (s *Service) CheckPortConflicts(ctx context.Context, requestedPorts []string) (*CheckPortsResult, error) {
	allocations, err := s.GetAllocatedPorts(ctx)
	if err != nil {
		// If Docker list fails, fall back to empty allocations and test OS listener directly
		allocations = []PortAllocation{}
	}

	result := &CheckPortsResult{
		HasConflict: false,
		Conflicts:   []PortConflict{},
	}

	for _, raw := range requestedPorts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		proto := "tcp"
		if idx := strings.LastIndex(raw, "/"); idx != -1 {
			proto = strings.ToLower(raw[idx+1:])
			raw = raw[:idx]
		}

		parts := splitPortString(raw)
		hostPort := 0
		containerPort := 0

		switch len(parts) {
		case 1:
			portStr := resolveEnvPortDefault(parts[0])
			cp, _ := strconv.Atoi(portStr)
			containerPort = cp
			hostPort = cp // Default host port to same as container port
		case 2:
			hp, _ := strconv.Atoi(resolveEnvPortDefault(parts[0]))
			cp, _ := strconv.Atoi(resolveEnvPortDefault(parts[1]))
			hostPort = hp
			containerPort = cp
		case 3:
			hp, _ := strconv.Atoi(resolveEnvPortDefault(parts[1]))
			cp, _ := strconv.Atoi(resolveEnvPortDefault(parts[2]))
			hostPort = hp
			containerPort = cp
		}

		if hostPort <= 0 {
			continue
		}

		inUse, conflictWith := s.IsPortInUse(hostPort, proto, allocations)
		suggested := hostPort
		if inUse {
			result.HasConflict = true
			suggested = s.FindNextAvailablePort(hostPort+1, proto, allocations)
		}

		result.Conflicts = append(result.Conflicts, PortConflict{
			HostPort:      hostPort,
			ContainerPort: containerPort,
			Protocol:      proto,
			InUse:         inUse,
			ConflictWith:  conflictWith,
			SuggestedPort: suggested,
		})
	}

	return result, nil
}
