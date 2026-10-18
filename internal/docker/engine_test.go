package docker_test

import (
	"context"
	"testing"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestGetEngineInfo(t *testing.T) {
	// Initialize with test or live service
	ds, err := docker.NewService("/var/run/docker.sock")
	if err != nil {
		t.Skip("docker daemon not available for live engine test")
	}

	info := ds.GetEngineInfo(context.Background())
	if info.Type == "" {
		t.Errorf("expected non-empty engine type")
	}
	if info.DisplayName == "" {
		t.Errorf("expected non-empty engine display name")
	}
	if info.ServerVersion == "" {
		t.Errorf("expected non-empty server version")
	}

	t.Logf("Detected engine: type=%s, name=%s, version=%s, serverVersion=%s",
		info.Type, info.DisplayName, info.Version, info.ServerVersion)
}
