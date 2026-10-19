package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

func TestExtractImageMetadataAndChangelog(t *testing.T) {
	ctx := context.Background()

	// 1. Image with OCI labels
	inspectWithLabels := types.ImageInspect{
		Config: &container.Config{
			Labels: map[string]string{
				"org.opencontainers.image.version": "v1.2.3",
				"org.opencontainers.image.source":  "https://github.com/caddyserver/caddy",
			},
		},
	}

	ver, src, cl, hub, _ := ExtractImageMetadataAndChangelog(ctx, "caddy:v1.2.3", inspectWithLabels)
	if ver != "v1.2.3" {
		t.Errorf("expected version v1.2.3, got %s", ver)
	}
	if src != "https://github.com/caddyserver/caddy" {
		t.Errorf("expected source caddyserver/caddy, got %s", src)
	}
	if cl != "https://github.com/caddyserver/caddy/releases" {
		t.Errorf("expected changelog url https://github.com/caddyserver/caddy/releases, got %s", cl)
	}
	if hub != "https://hub.docker.com/_/caddy" {
		t.Errorf("expected hub url https://hub.docker.com/_/caddy, got %s", hub)
	}

	// 2. Known upstream fallback for official images
	inspectEmpty := types.ImageInspect{}
	ver2, src2, cl2, hub2, _ := ExtractImageMetadataAndChangelog(ctx, "golang:alpine", inspectEmpty)
	if ver2 != "alpine" {
		t.Errorf("expected tag fallback 'alpine', got %s", ver2)
	}
	if src2 != "https://github.com/golang/go" {
		t.Errorf("expected golang/go upstream source, got %s", src2)
	}
	if cl2 != "https://github.com/golang/go/releases" {
		t.Errorf("expected golang/go releases changelog, got %s", cl2)
	}
	if hub2 != "https://hub.docker.com/_/golang" {
		t.Errorf("expected hub url https://hub.docker.com/_/golang, got %s", hub2)
	}
}
