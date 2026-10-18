package docker_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestImageValidation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()

	// PullImage with empty / whitespace ref
	for _, ref := range []string{"", "   ", "\t"} {
		reader, err := svc.PullImage(ctx, ref)
		if err == nil {
			if reader != nil {
				_ = reader.Close()
			}
			t.Errorf("PullImage(%q): expected error, got nil", ref)
		}
		if !errors.Is(err, docker.ErrImageRefRequired) {
			t.Errorf("PullImage(%q): expected ErrImageRefRequired, got %v", ref, err)
		}
	}

	// RemoveImage with empty / whitespace ID
	for _, id := range []string{"", "   ", "\t"} {
		err := svc.RemoveImage(ctx, id, false)
		if err == nil {
			t.Errorf("RemoveImage(%q): expected error, got nil", id)
		}
		if !errors.Is(err, docker.ErrImageIDRequired) {
			t.Errorf("RemoveImage(%q): expected ErrImageIDRequired, got %v", id, err)
		}
	}
}

func TestVolumeValidation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()

	// CreateVolume with empty / whitespace name
	for _, name := range []string{"", "   ", "\t"} {
		vol, err := svc.CreateVolume(ctx, name, nil)
		if err == nil {
			t.Errorf("CreateVolume(%q): expected error, got nil (vol=%+v)", name, vol)
		}
		if !errors.Is(err, docker.ErrVolumeNameRequired) {
			t.Errorf("CreateVolume(%q): expected ErrVolumeNameRequired, got %v", name, err)
		}
	}

	// RemoveVolume with empty / whitespace name
	for _, name := range []string{"", "   ", "\t"} {
		err := svc.RemoveVolume(ctx, name, false)
		if err == nil {
			t.Errorf("RemoveVolume(%q): expected error, got nil", name)
		}
		if !errors.Is(err, docker.ErrVolumeNameRequired) {
			t.Errorf("RemoveVolume(%q): expected ErrVolumeNameRequired, got %v", name, err)
		}
	}
}

func TestNetworkValidation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()

	// CreateNetwork with empty / whitespace name
	for _, name := range []string{"", "   ", "\t"} {
		netID, err := svc.CreateNetwork(ctx, name, "bridge")
		if err == nil {
			t.Errorf("CreateNetwork(%q): expected error, got nil (netID=%s)", name, netID)
		}
		if !errors.Is(err, docker.ErrNetworkNameRequired) {
			t.Errorf("CreateNetwork(%q): expected ErrNetworkNameRequired, got %v", name, err)
		}
	}

	// RemoveNetwork with empty / whitespace ID
	for _, id := range []string{"", "   ", "\t"} {
		err := svc.RemoveNetwork(ctx, id)
		if err == nil {
			t.Errorf("RemoveNetwork(%q): expected error, got nil", id)
		}
		if !errors.Is(err, docker.ErrNetworkIDRequired) {
			t.Errorf("RemoveNetwork(%q): expected ErrNetworkIDRequired, got %v", id, err)
		}
	}
}

func TestDockerResourcesLive(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible, skipping live resource tests: %v", err)
	}

	// Test ListImages
	images, err := svc.ListImages(ctx)
	if err != nil {
		t.Fatalf("ListImages failed: %v", err)
	}
	t.Logf("ListImages returned %d images", len(images))

	// Test ListVolumes
	volumes, err := svc.ListVolumes(ctx)
	if err != nil {
		t.Fatalf("ListVolumes failed: %v", err)
	}
	t.Logf("ListVolumes returned %d volumes", len(volumes.Volumes))
	for i, v := range volumes.Volumes {
		if i >= 3 {
			break
		}
		if v.UsageData != nil {
			t.Logf("Volume %s size: %d bytes (ref: %d)", v.Name, v.UsageData.Size, v.UsageData.RefCount)
		} else {
			t.Logf("Volume %s has nil UsageData", v.Name)
		}
	}

	// Test Volume Lifecycle: Create -> List -> Remove
	testVolName := fmt.Sprintf("pdswarm-test-vol-%d", time.Now().UnixNano())
	createdVol, err := svc.CreateVolume(ctx, testVolName, map[string]string{"test": "pdswarm"})
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}
	if createdVol.Name != testVolName {
		t.Errorf("expected volume name %q, got %q", testVolName, createdVol.Name)
	}

	if err := svc.RemoveVolume(ctx, testVolName, true); err != nil {
		t.Fatalf("RemoveVolume failed: %v", err)
	}

	// Test ListNetworks
	networks, err := svc.ListNetworks(ctx)
	if err != nil {
		t.Fatalf("ListNetworks failed: %v", err)
	}
	t.Logf("ListNetworks returned %d networks", len(networks))

	// Test Network Lifecycle: Create -> List -> Remove
	testNetName := fmt.Sprintf("pdswarm-test-net-%d", time.Now().UnixNano())
	netID, err := svc.CreateNetwork(ctx, testNetName, "bridge")
	if err != nil {
		t.Fatalf("CreateNetwork failed: %v", err)
	}
	if netID == "" {
		t.Fatalf("expected non-empty network ID")
	}

	if err := svc.RemoveNetwork(ctx, netID); err != nil {
		t.Fatalf("RemoveNetwork failed: %v", err)
	}
}

func TestDockerResourcesContextCancellation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	if _, err := svc.ListImages(ctx); err == nil {
		t.Error("expected error for ListImages with canceled context")
	}
	if _, err := svc.PullImage(ctx, "alpine:latest"); err == nil {
		t.Error("expected error for PullImage with canceled context")
	}
	if err := svc.RemoveImage(ctx, "test-image", false); err == nil {
		t.Error("expected error for RemoveImage with canceled context")
	}
	if _, err := svc.PruneImages(ctx); err == nil {
		t.Error("expected error for PruneImages with canceled context")
	}

	if _, err := svc.ListVolumes(ctx); err == nil {
		t.Error("expected error for ListVolumes with canceled context")
	}
	if _, err := svc.CreateVolume(ctx, "test-vol", nil); err == nil {
		t.Error("expected error for CreateVolume with canceled context")
	}
	if err := svc.RemoveVolume(ctx, "test-vol", false); err == nil {
		t.Error("expected error for RemoveVolume with canceled context")
	}
	if _, err := svc.PruneVolumes(ctx); err == nil {
		t.Error("expected error for PruneVolumes with canceled context")
	}

	if _, err := svc.ListNetworks(ctx); err == nil {
		t.Error("expected error for ListNetworks with canceled context")
	}
	if _, err := svc.CreateNetwork(ctx, "test-net", "bridge"); err == nil {
		t.Error("expected error for CreateNetwork with canceled context")
	}
	if err := svc.RemoveNetwork(ctx, "test-net"); err == nil {
		t.Error("expected error for RemoveNetwork with canceled context")
	}
	if _, err := svc.PruneNetworks(ctx); err == nil {
		t.Error("expected error for PruneNetworks with canceled context")
	}
}

func TestEncodeRegistryAuth(t *testing.T) {
	// Empty credentials return empty string
	if auth := docker.EncodeRegistryAuth("", "", ""); auth != "" {
		t.Errorf("expected empty string for empty creds, got %q", auth)
	}

	// Valid credentials return non-empty base64 string
	auth := docker.EncodeRegistryAuth("testuser", "testpassword", "https://index.docker.io/v1/")
	if auth == "" {
		t.Fatal("expected encoded auth string, got empty")
	}

	// Verify PullImageStream with empty ref returns ErrImageRefRequired
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	if _, err := svc.PullImageStream(context.Background(), "", auth); !errors.Is(err, docker.ErrImageRefRequired) {
		t.Errorf("expected ErrImageRefRequired, got %v", err)
	}
}

