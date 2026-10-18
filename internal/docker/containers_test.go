package docker_test

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/podiax/pdswarm/internal/docker"
)

func TestValidateCreateRequest(t *testing.T) {
	req := docker.CreateContainerRequest{
		Image: "",
		Name:  "my-nginx",
	}
	err := req.Validate()
	if err == nil {
		t.Fatalf("expected error for empty image name")
	}

	req.Image = "nginx:alpine"
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}
}

func TestNewService(t *testing.T) {
	// Test default socket
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("unexpected error creating service with default socket: %v", err)
	}
	if svc == nil {
		t.Fatalf("expected non-nil service")
	}
	if svc.RawClient() == nil {
		t.Fatalf("expected non-nil RawClient")
	}
	if err := svc.Close(); err != nil {
		t.Errorf("error closing service: %v", err)
	}

	// Test normalization of path without URI scheme
	svcPath, err := docker.NewService("/var/run/docker.sock")
	if err != nil {
		t.Fatalf("unexpected error creating service with plain socket path: %v", err)
	}
	if svcPath == nil {
		t.Fatalf("expected non-nil service")
	}
	_ = svcPath.Close()

	// Test normalization of custom path without URI scheme
	svcCustomPath, err := docker.NewService("/custom/path.sock")
	if err != nil {
		t.Fatalf("unexpected error creating service with custom socket path: %v", err)
	}
	if svcCustomPath == nil {
		t.Fatalf("expected non-nil service")
	}
	_ = svcCustomPath.Close()

	// Test custom socket with explicit URI scheme
	svcCustomURI, err := docker.NewService("unix:///tmp/custom-docker.sock")
	if err != nil {
		t.Fatalf("unexpected error creating service with custom socket URI: %v", err)
	}
	if svcCustomURI == nil {
		t.Fatalf("expected non-nil service")
	}
	_ = svcCustomURI.Close()
}

func TestCreateContainer_Validation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()

	// Empty image error
	_, err = svc.CreateContainer(ctx, docker.CreateContainerRequest{
		Image: "",
	})
	if err == nil {
		t.Fatal("expected error creating container without image")
	}

	// Invalid port error
	_, err = svc.CreateContainer(ctx, docker.CreateContainerRequest{
		Image: "alpine",
		Ports: []docker.PortMapping{
			{ContainerPort: "invalid-port", HostPort: "8080", Protocol: "tcp"},
		},
	})
	if err == nil {
		t.Fatal("expected error with invalid port mapping")
	}
}

func TestContainerToCreateRequest(t *testing.T) {
	containerJSON := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: "/my-app-web",
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{
					Name: "unless-stopped",
				},
				Resources: container.Resources{
					Memory:   1024 * 1024 * 512, // 512 MB
					NanoCPUs: 1500000000,         // 1.5 cores
				},
				Binds: []string{
					"/host/data:/container/data:rw",
					"/host/config:/container/config:ro",
				},
				PortBindings: nat.PortMap{
					nat.Port("80/tcp"): []nat.PortBinding{
						{HostIP: "0.0.0.0", HostPort: "8080"},
					},
					nat.Port("443/tcp"): []nat.PortBinding{
						{HostIP: "0.0.0.0", HostPort: "8443"},
					},
				},
			},
		},
		Mounts: []types.MountPoint{
			{
				Source:      "/var/log",
				Destination: "/app/log",
				RW:          true,
			},
			{
				Source:      "/host/data",
				Destination: "/container/data",
				RW:          true, // Duplicate of bind, should be deduplicated
			},
		},
		Config: &container.Config{
			Image: "nginx:alpine",
			Env: []string{
				"ENV=production",
				"PORT=80",
			},
			Labels: map[string]string{
				"app":     "test",
				"version": "1.0",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			NetworkSettingsBase: types.NetworkSettingsBase{
				Ports: nat.PortMap{
					nat.Port("9000/tcp"): []nat.PortBinding{
						{HostIP: "0.0.0.0", HostPort: "9000"},
					},
				},
			},
		},
	}

	req := docker.ContainerToCreateRequest(containerJSON)

	// Check Name trimming
	if req.Name != "my-app-web" {
		t.Errorf("expected name 'my-app-web', got '%s'", req.Name)
	}

	// Check Image
	if req.Image != "nginx:alpine" {
		t.Errorf("expected image 'nginx:alpine', got '%s'", req.Image)
	}

	// Check Restart Policy
	if req.RestartPolicy != "unless-stopped" {
		t.Errorf("expected restart policy 'unless-stopped', got '%s'", req.RestartPolicy)
	}

	// Check MemoryMB
	if req.MemoryMB != 512 {
		t.Errorf("expected MemoryMB 512, got %d", req.MemoryMB)
	}

	// Check CPUCores
	if req.CPUCores != 1.5 {
		t.Errorf("expected CPUCores 1.5, got %f", req.CPUCores)
	}

	// Check Env
	if len(req.Env) != 2 || req.Env[0] != "ENV=production" || req.Env[1] != "PORT=80" {
		t.Errorf("expected Env [ENV=production PORT=80], got %v", req.Env)
	}

	// Check Labels
	if len(req.Labels) != 2 || req.Labels["app"] != "test" || req.Labels["version"] != "1.0" {
		t.Errorf("expected labels app=test, version=1.0, got %v", req.Labels)
	}

	// Check Volumes
	if len(req.Volumes) != 3 {
		t.Fatalf("expected 3 volumes (deduplicated), got %d: %v", len(req.Volumes), req.Volumes)
	}
	volMap := make(map[string]docker.VolumeMount)
	for _, v := range req.Volumes {
		volMap[v.HostPath+":"+v.ContainerPath] = v
	}
	if v, ok := volMap["/host/config:/container/config"]; !ok || !v.ReadOnly {
		t.Errorf("expected /host/config to be read-only volume mount, got %+v", v)
	}
	if v, ok := volMap["/host/data:/container/data"]; !ok || v.ReadOnly {
		t.Errorf("expected /host/data to be read-write volume mount, got %+v", v)
	}
	if v, ok := volMap["/var/log:/app/log"]; !ok || v.ReadOnly {
		t.Errorf("expected /var/log to be read-write volume mount, got %+v", v)
	}

	// Check Ports (2 from HostConfig, 1 from NetworkSettings)
	if len(req.Ports) != 3 {
		t.Fatalf("expected 3 ports, got %d: %v", len(req.Ports), req.Ports)
	}
	portMap := make(map[string]docker.PortMapping)
	for _, p := range req.Ports {
		portMap[p.ContainerPort+"/"+p.Protocol] = p
	}
	if p, ok := portMap["80/tcp"]; !ok || p.HostPort != "8080" {
		t.Errorf("expected port 80/tcp -> 8080, got %+v", p)
	}
	if p, ok := portMap["443/tcp"]; !ok || p.HostPort != "8443" {
		t.Errorf("expected port 443/tcp -> 8443, got %+v", p)
	}
	if p, ok := portMap["9000/tcp"]; !ok || p.HostPort != "9000" {
		t.Errorf("expected port 9000/tcp -> 9000, got %+v", p)
	}
}

func TestContainerToCreateRequest_NilSafety(t *testing.T) {
	// Minimal/empty container
	c := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: "test",
		},
	}
	req := docker.ContainerToCreateRequest(c)
	if req.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", req.Name)
	}
	if req.Image != "" {
		t.Errorf("expected empty image, got '%s'", req.Image)
	}
	if len(req.Ports) != 0 || len(req.Volumes) != 0 || len(req.Env) != 0 {
		t.Errorf("expected empty slices, got ports=%v volumes=%v env=%v", req.Ports, req.Volumes, req.Env)
	}
}

func TestFilterUserEnv(t *testing.T) {
	containerEnv := []string{
		"PATH=/usr/local/bin:/usr/bin",
		"GOSU_VERSION=1.19",
		"LANG=en_US.utf8",
		"PG_VERSION=16.15",
		"POSTGRES_DB=app_db",
		"POSTGRES_USER=postgres",
		"POSTGRES_PASSWORD=postgres",
		"OVERRIDDEN_LANG=fr_FR.utf8",
	}

	imageEnv := []string{
		"PATH=/usr/local/bin:/usr/bin",
		"GOSU_VERSION=1.19",
		"LANG=en_US.utf8",
		"PG_VERSION=16.15",
		"OVERRIDDEN_LANG=en_US.utf8",
	}

	filtered := docker.FilterUserEnv(containerEnv, imageEnv)

	expected := []string{
		"POSTGRES_DB=app_db",
		"POSTGRES_USER=postgres",
		"POSTGRES_PASSWORD=postgres",
		"OVERRIDDEN_LANG=fr_FR.utf8",
	}

	if len(filtered) != len(expected) {
		t.Fatalf("expected %d user env vars, got %d: %v", len(expected), len(filtered), filtered)
	}

	for i, v := range expected {
		if filtered[i] != v {
			t.Errorf("at index %d: expected %s, got %s", i, v, filtered[i])
		}
	}

	// Test empty imageEnv preserves all
	allPreserved := docker.FilterUserEnv(containerEnv, nil)
	if len(allPreserved) != len(containerEnv) {
		t.Errorf("expected all %d vars preserved when imageEnv is nil, got %d", len(containerEnv), len(allPreserved))
	}
}

func TestContainerToCreateRequestWithImageEnv(t *testing.T) {
	c := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			Name: "/my-postgres",
		},
		Config: &container.Config{
			Image: "postgres:16-alpine",
			Env: []string{
				"PATH=/usr/bin",
				"PG_VERSION=16.15",
				"POSTGRES_DB=mydb",
			},
		},
	}

	imageEnv := []string{
		"PATH=/usr/bin",
		"PG_VERSION=16.15",
	}

	req := docker.ContainerToCreateRequestWithImageEnv(c, imageEnv)
	if len(req.Env) != 1 || req.Env[0] != "POSTGRES_DB=mydb" {
		t.Errorf("expected only 'POSTGRES_DB=mydb', got: %v", req.Env)
	}
}

