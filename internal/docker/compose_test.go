package docker_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func TestStackLabels(t *testing.T) {
	projectName := "my-stack"
	serviceName := "web"

	labels := docker.StackLabels(projectName, serviceName)

	if labels[docker.ComposeProjectLabel] != projectName {
		t.Errorf("expected project label %q, got %q", projectName, labels[docker.ComposeProjectLabel])
	}
	if labels[docker.ComposeServiceLabel] != serviceName {
		t.Errorf("expected service label %q, got %q", serviceName, labels[docker.ComposeServiceLabel])
	}
	if labels[docker.ComposeVersionLabel] != "pdswarm" {
		t.Errorf("expected version label %q, got %q", "pdswarm", labels[docker.ComposeVersionLabel])
	}
}

func TestParseComposeYAML(t *testing.T) {
	yamlContent := `
version: "3.8"
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
      - "443:443"
      - "8081:81/udp"
    environment:
      - FOO=BAR
      - DEBUG=1
    volumes:
      - /host/path:/container/path:ro
      - data-volume:/var/data
    command: ["nginx", "-g", "daemon off;"]
    restart: always
    networks:
      - front
  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: testdb
    ports:
      - 5432
    command: postgres -c shared_buffers=256MB
    restart: unless-stopped
    networks:
      - back
networks:
  front:
  back:
`

	cfg, err := docker.ParseComposeYAML(yamlContent)
	if err != nil {
		t.Fatalf("failed to parse compose YAML: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	web, ok := cfg.Services["web"]
	if !ok {
		t.Fatalf("expected service 'web' to exist")
	}
	if web.Image != "nginx:alpine" {
		t.Errorf("expected image 'nginx:alpine', got %q", web.Image)
	}
	if len(web.Ports) != 3 {
		t.Fatalf("expected 3 ports for web, got %d", len(web.Ports))
	}
	if web.Ports[0].HostPort != "8080" || web.Ports[0].ContainerPort != "80" || web.Ports[0].Protocol != "tcp" {
		t.Errorf("unexpected port mapping: %+v", web.Ports[0])
	}
	if web.Ports[2].Protocol != "udp" {
		t.Errorf("expected udp protocol for port 3, got %s", web.Ports[2].Protocol)
	}

	if len(web.Environment) != 2 {
		t.Fatalf("expected 2 env vars for web, got %d", len(web.Environment))
	}
	if web.Environment[0] != "FOO=BAR" || web.Environment[1] != "DEBUG=1" {
		t.Errorf("unexpected env: %v", web.Environment)
	}

	if len(web.Volumes) != 2 {
		t.Fatalf("expected 2 volumes for web, got %d", len(web.Volumes))
	}
	if !web.Volumes[0].ReadOnly {
		t.Errorf("expected first volume to be read-only")
	}

	if len(web.Command) != 3 || web.Command[0] != "nginx" {
		t.Errorf("unexpected command slice: %v", web.Command)
	}
	if web.Restart != "always" {
		t.Errorf("expected restart 'always', got %s", web.Restart)
	}
	if len(web.Networks) != 1 || web.Networks[0] != "front" {
		t.Errorf("unexpected networks: %v", web.Networks)
	}

	db, ok := cfg.Services["db"]
	if !ok {
		t.Fatalf("expected service 'db' to exist")
	}
	if db.Image != "postgres:15" {
		t.Errorf("expected image 'postgres:15', got %q", db.Image)
	}
	// Check map environment unmarshaling
	if len(db.Environment) != 2 {
		t.Fatalf("expected 2 env vars for db, got %d", len(db.Environment))
	}
	// Port 5432 as scalar int
	if len(db.Ports) != 1 || db.Ports[0].ContainerPort != "5432" {
		t.Errorf("unexpected port for db: %+v", db.Ports)
	}
	// String command
	if len(db.Command) == 0 || db.Command[0] != "postgres" {
		t.Errorf("unexpected command for db: %v", db.Command)
	}
	if db.Restart != "unless-stopped" {
		t.Errorf("expected restart 'unless-stopped', got %s", db.Restart)
	}

	if len(cfg.Networks) != 2 {
		t.Errorf("expected 2 networks, got %d", len(cfg.Networks))
	}
}

func TestParseComposeYAML_Validation(t *testing.T) {
	// Empty content
	_, err := docker.ParseComposeYAML("")
	if err == nil {
		t.Errorf("expected error for empty yaml")
	}

	// Invalid syntax
	_, err = docker.ParseComposeYAML(":::invalid yaml:::")
	if err == nil {
		t.Errorf("expected error for malformed yaml")
	}

	// No services
	_, err = docker.ParseComposeYAML("version: '3.8'\nnetworks:\n  foo:\n")
	if err == nil {
		t.Errorf("expected error when no services defined")
	}

	// Service without image
	_, err = docker.ParseComposeYAML("services:\n  app:\n    restart: always\n")
	if err == nil {
		t.Errorf("expected error when service is missing image")
	}
}

func TestService_ComposeValidation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()

	// DeployStack with nil stack
	if err := svc.DeployStack(ctx, nil); err == nil {
		t.Errorf("expected error for nil stack")
	}

	// DeployStack with empty name
	if err := svc.DeployStack(ctx, &store.Stack{Name: "", ComposeYAML: "services:\n  app:\n    image: nginx"}); err == nil {
		t.Errorf("expected error for empty stack name")
	}

	// DeployStack with empty YAML
	if err := svc.DeployStack(ctx, &store.Stack{Name: "test", ComposeYAML: ""}); err == nil {
		t.Errorf("expected error for empty compose yaml")
	}

	// StopStack with empty name
	if err := svc.StopStack(ctx, ""); err == nil {
		t.Errorf("expected error for empty name in StopStack")
	}

	// DownStack with empty name
	if err := svc.DownStack(ctx, ""); err == nil {
		t.Errorf("expected error for empty name in DownStack")
	}

	// GetStackContainers with empty name
	if _, err := svc.GetStackContainers(ctx, ""); err == nil {
		t.Errorf("expected error for empty name in GetStackContainers")
	}
}

func TestParseComposeYAML_AnonymousVolumes(t *testing.T) {
	yamlContent := `
version: "3.8"
services:
  db:
    image: postgres:16
    volumes:
      - /var/lib/postgresql/data
      - ./custom.conf:/etc/postgresql/postgresql.conf:ro
`
	cfg, err := docker.ParseComposeYAML(yamlContent)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	db := cfg.Services["db"]
	if len(db.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(db.Volumes))
	}

	// First volume is anonymous
	if db.Volumes[0].HostPath != "" {
		t.Errorf("expected empty host path for anonymous volume, got %q", db.Volumes[0].HostPath)
	}
	if db.Volumes[0].ContainerPath != "/var/lib/postgresql/data" {
		t.Errorf("unexpected container path: %q", db.Volumes[0].ContainerPath)
	}

	// Second volume has host bind
	if db.Volumes[1].HostPath != "./custom.conf" {
		t.Errorf("expected host path './custom.conf', got %q", db.Volumes[1].HostPath)
	}
	if !db.Volumes[1].ReadOnly {
		t.Errorf("expected read only flag to be true")
	}
}

func TestSynthesizeComposeYAML(t *testing.T) {
	ctx := context.Background()

	containers := []docker.ContainerSummary{
		{
			ID:    "c111111111111",
			Names: []string{"/immich_server"},
			Image: "ghcr.io/immich-app/immich-server:latest",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "immich-new",
				docker.ComposeServiceLabel: "immich-server",
			},
			Ports: []types.Port{
				{PublicPort: 2283, PrivatePort: 2283, Type: "tcp"},
			},
			Mounts: []types.MountPoint{
				{
					Type:        "bind",
					Source:      "/mnt/nvme_data/immich/upload",
					Destination: "/usr/src/app/upload",
					RW:          true,
				},
			},
		},
		{
			ID:    "c222222222222",
			Names: []string{"/immich_postgres"},
			Image: "ghcr.io/immich-app/postgres:14-alpine",
			Labels: map[string]string{
				docker.ComposeProjectLabel: "immich-new",
				docker.ComposeServiceLabel: "database",
			},
			Ports: []types.Port{
				{PublicPort: 5432, PrivatePort: 5432, Type: "tcp"},
			},
		},
	}

	yamlStr, err := docker.SynthesizeComposeYAML(ctx, nil, "immich-new", containers)
	if err != nil {
		t.Fatalf("SynthesizeComposeYAML failed: %v", err)
	}

	if len(yamlStr) == 0 {
		t.Fatalf("expected non-empty YAML")
	}

	// Verify the synthesized YAML can be parsed cleanly by ParseComposeYAML
	cfg, err := docker.ParseComposeYAML(yamlStr)
	if err != nil {
		t.Fatalf("synthesized YAML is invalid Compose YAML: %v\nYAML content:\n%s", err, yamlStr)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	serverSvc, ok := cfg.Services["immich-server"]
	if !ok {
		t.Fatalf("expected service 'immich-server'")
	}
	if serverSvc.Image != "ghcr.io/immich-app/immich-server:latest" {
		t.Errorf("unexpected image: %s", serverSvc.Image)
	}
	if serverSvc.ContainerName != "immich_server" {
		t.Errorf("unexpected container_name: %s", serverSvc.ContainerName)
	}
	if len(serverSvc.Ports) != 1 || serverSvc.Ports[0].HostPort != "2283" {
		t.Errorf("expected port 2283, got: %v", serverSvc.Ports)
	}

	dbSvc, ok := cfg.Services["database"]
	if !ok {
		t.Fatalf("expected service 'database'")
	}
	if dbSvc.Image != "ghcr.io/immich-app/postgres:14-alpine" {
		t.Errorf("unexpected image: %s", dbSvc.Image)
	}
}

func TestExtractOrSynthesizeStackEnv_Disk(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	envFile := tmpDir + "/stack.env"
	if err := os.WriteFile(envFile, []byte("PUID=1000\nIMMICH_VERSION=release\n"), 0644); err != nil {
		t.Fatalf("failed to write test env file: %v", err)
	}

	svc := docker.NewServiceWithClient(nil)
	recovered := svc.ExtractOrSynthesizeStackEnv(ctx, tmpDir, nil)
	if !strings.Contains(recovered, "PUID=1000") || !strings.Contains(recovered, "IMMICH_VERSION=release") {
		t.Errorf("failed to recover stack.env from disk: got %q", recovered)
	}
}


