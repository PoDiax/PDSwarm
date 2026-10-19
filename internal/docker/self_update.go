package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// FindSelfContainer attempts to locate the container in which the current PDSwarm process is running.
func (s *Service) FindSelfContainer(ctx context.Context) (*types.ContainerJSON, error) {
	if s == nil || s.cli == nil {
		return nil, errors.New("docker client not available")
	}

	// 1. In Docker, hostname defaults to the short container ID
	hostname, _ := os.Hostname()
	if hostname != "" {
		if c, err := s.InspectContainer(ctx, hostname); err == nil && c.ContainerJSONBase != nil {
			return &c, nil
		}
	}

	// 2. Fallback: Search all containers for matching ID prefix, image name, or container name
	containers, err := s.ListContainers(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		if hostname != "" && strings.HasPrefix(c.ID, hostname) {
			if inspected, err := s.InspectContainer(ctx, c.ID); err == nil {
				return &inspected, nil
			}
		}
		if strings.Contains(strings.ToLower(c.Image), "pdswarm") {
			if inspected, err := s.InspectContainer(ctx, c.ID); err == nil {
				return &inspected, nil
			}
		}
		for _, name := range c.Names {
			if strings.Contains(strings.ToLower(name), "pdswarm") {
				if inspected, err := s.InspectContainer(ctx, c.ID); err == nil {
					return &inspected, nil
				}
			}
		}
	}

	return nil, errors.New("unable to identify own PDSwarm container")
}

// SelfUpdateContainer pulls the target image and launches an ephemeral helper container
// that stops the old PDSwarm container, removes it, and runs the updated image with identical configuration.
func (s *Service) SelfUpdateContainer(ctx context.Context, targetImage string) error {
	self, err := s.FindSelfContainer(ctx)
	if err != nil {
		return fmt.Errorf("failed to locate running PDSwarm container: %w", err)
	}

	if targetImage == "" {
		targetImage = self.Config.Image
	}
	if targetImage == "" || targetImage == "pdswarm" {
		targetImage = "ghcr.io/podiax/pdswarm:latest"
	}

	// 1. Pull the new image version
	log.Printf("[INFO] Pulling target update image %s...", targetImage)
	rc, err := s.PullImage(ctx, targetImage)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", targetImage, err)
	}
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()

	// 2. Pull the helper CLI image (docker:cli)
	log.Printf("[INFO] Ensuring helper image docker:cli is available...")
	if rcHelper, err := s.PullImage(ctx, "docker:cli"); err == nil {
		_, _ = io.Copy(io.Discard, rcHelper)
		_ = rcHelper.Close()
	}

	cleanName := strings.TrimPrefix(self.Name, "/")
	cleanID := self.ID

	// 3. Synthesize docker run flags to recreate the container faithfully
	var runArgs []string

	if cleanName != "" {
		runArgs = append(runArgs, fmt.Sprintf("--name %q", cleanName))
	}

	if self.HostConfig != nil {
		// Restart policy
		if self.HostConfig.RestartPolicy.Name != "" {
			runArgs = append(runArgs, fmt.Sprintf("--restart %s", self.HostConfig.RestartPolicy.Name))
		}

		// Network mode
		netMode := string(self.HostConfig.NetworkMode)
		if netMode != "" && netMode != "default" && netMode != "bridge" {
			runArgs = append(runArgs, fmt.Sprintf("--network %q", netMode))
		}

		// Port bindings
		for portProto, bindings := range self.HostConfig.PortBindings {
			for _, b := range bindings {
				if b.HostIP != "" && b.HostIP != "0.0.0.0" {
					runArgs = append(runArgs, fmt.Sprintf("-p %s:%s:%s", b.HostIP, b.HostPort, portProto))
				} else if b.HostPort != "" {
					runArgs = append(runArgs, fmt.Sprintf("-p %s:%s", b.HostPort, portProto))
				}
			}
		}

		// Volume mounts / binds
		for _, bind := range self.HostConfig.Binds {
			runArgs = append(runArgs, fmt.Sprintf("-v %q", bind))
		}
	}

	// Environment variables
	if self.Config != nil {
		for _, env := range self.Config.Env {
			// Skip internal Docker container hostname/HOME noise
			if strings.HasPrefix(env, "HOSTNAME=") || strings.HasPrefix(env, "SHLVL=") {
				continue
			}
			runArgs = append(runArgs, fmt.Sprintf("-e %q", env))
		}

		// Labels (preserves docker-compose project/service affiliations if any)
		for k, v := range self.Config.Labels {
			runArgs = append(runArgs, fmt.Sprintf("-l %q", fmt.Sprintf("%s=%s", k, v)))
		}
	}

	runArgs = append(runArgs, targetImage)

	if self.Config != nil && len(self.Config.Cmd) > 0 {
		for _, arg := range self.Config.Cmd {
			runArgs = append(runArgs, fmt.Sprintf("%q", arg))
		}
	}

	runCommand := strings.Join(runArgs, " ")
	helperScript := fmt.Sprintf("sleep 2 && docker stop %s && docker rm %s && docker run -d %s", cleanID, cleanID, runCommand)

	helperName := fmt.Sprintf("pdswarm-updater-%d", time.Now().Unix())

	cfg := &container.Config{
		Image: "docker:cli",
		Cmd:   []string{"sh", "-c", helperScript},
	}
	hcfg := &container.HostConfig{
		Binds:      []string{"/var/run/docker.sock:/var/run/docker.sock"},
		AutoRemove: true,
	}

	resp, err := s.cli.ContainerCreate(ctx, cfg, hcfg, nil, nil, helperName)
	if err != nil {
		return fmt.Errorf("failed to create updater helper container: %w", err)
	}

	if err := s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start updater helper container: %w", err)
	}

	log.Printf("[INFO] Ephemeral updater helper %s launched successfully. Old container %s will be replaced shortly.", helperName, cleanID[:12])
	return nil
}
