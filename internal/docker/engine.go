package docker

import (
	"context"
	"strings"
)

// EngineInfo holds metadata about the underlying container engine (Docker or Podman).
type EngineInfo struct {
	Type          string `json:"type"`           // "docker" or "podman"
	DisplayName   string `json:"display_name"`   // "Docker Engine" or "Podman"
	Version       string `json:"version"`        // e.g. "29.9.1" or "5.0.3"
	ServerVersion string `json:"server_version"` // e.g. "Docker Engine 29.9.1" or "Podman 5.0.3"
}

// GetEngineInfo interrogates the daemon and returns whether it is Docker or Podman.
func (s *Service) GetEngineInfo(ctx context.Context) EngineInfo {
	engineType := "docker"
	displayName := "Docker Engine"
	version := ""
	serverVersion := ""

	// Check daemon host string for "podman"
	if s.cli != nil {
		hostStr := strings.ToLower(s.cli.DaemonHost())
		if strings.Contains(hostStr, "podman") {
			engineType = "podman"
			displayName = "Podman"
		}
	}

	// 1. Inspect ServerVersion if available
	ver, err := s.cli.ServerVersion(ctx)
	if err == nil {
		if ver.Version != "" {
			version = ver.Version
			serverVersion = ver.Version
		}

		// Check Platform Name
		if strings.Contains(strings.ToLower(ver.Platform.Name), "podman") {
			engineType = "podman"
			displayName = "Podman"
		}

		// Check Components
		for _, comp := range ver.Components {
			cName := strings.ToLower(comp.Name)
			if strings.Contains(cName, "podman") {
				engineType = "podman"
				displayName = "Podman"
				if comp.Version != "" {
					version = comp.Version
				}
				break
			}
		}
	}

	// 2. Inspect System Info for additional clues
	info, err := s.cli.Info(ctx)
	if err == nil {
		if version == "" {
			version = info.ServerVersion
		}
		if serverVersion == "" {
			serverVersion = info.ServerVersion
		}

		svLower := strings.ToLower(info.ServerVersion)
		if strings.Contains(svLower, "podman") {
			engineType = "podman"
			displayName = "Podman"
		}

		if strings.Contains(strings.ToLower(info.InitBinary), "podman") ||
			strings.Contains(strings.ToLower(info.Name), "podman") {
			engineType = "podman"
			displayName = "Podman"
		}
	}

	// Clean up and format ServerVersion string
	if engineType == "podman" {
		if !strings.HasPrefix(strings.ToLower(serverVersion), "podman") {
			if version != "" {
				serverVersion = "Podman " + version
			} else {
				serverVersion = "Podman"
			}
		}
	} else {
		if !strings.HasPrefix(strings.ToLower(serverVersion), "docker") {
			if version != "" {
				serverVersion = "Docker Engine " + version
			} else {
				serverVersion = "Docker Engine"
			}
		}
	}

	return EngineInfo{
		Type:          engineType,
		DisplayName:   displayName,
		Version:       version,
		ServerVersion: serverVersion,
	}
}
