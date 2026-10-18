package docker_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestBuildImageStream_Validation(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()

	// Empty tag
	for _, tag := range []string{"", "   ", "\t"} {
		stream, err := svc.BuildImageStream(ctx, tag, "FROM alpine:latest")
		if err == nil {
			if stream != nil {
				_ = stream.Close()
			}
			t.Errorf("BuildImageStream with tag %q: expected error, got nil", tag)
		}
		if !errors.Is(err, docker.ErrTagRequired) {
			t.Errorf("BuildImageStream with tag %q: expected ErrTagRequired, got %v", tag, err)
		}
	}

	// Empty dockerfile
	for _, df := range []string{"", "   ", "\n\t\n"} {
		stream, err := svc.BuildImageStream(ctx, "test-tag:latest", df)
		if err == nil {
			if stream != nil {
				_ = stream.Close()
			}
			t.Errorf("BuildImageStream with empty dockerfile: expected error, got nil")
		}
		if !errors.Is(err, docker.ErrDockerfileRequired) {
			t.Errorf("BuildImageStream with empty dockerfile: expected ErrDockerfileRequired, got %v", err)
		}
	}
}

func TestBuildImageStream_NilClient(t *testing.T) {
	svc := docker.NewServiceWithClient(nil)
	ctx := context.Background()

	_, err := svc.BuildImageStream(ctx, "test:latest", "FROM alpine")
	if err == nil {
		t.Fatalf("expected error with nil client, got nil")
	}
}

func TestBuildImageStream_LiveBuild(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Skip("docker not available")
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("docker ping failed: %v", err)
	}

	testTag := "pdswarm-test-builder:latest"
	dockerfile := "FROM scratch\nLABEL org.opencontainers.image.title=\"pdswarm-test\"\n"

	stream, err := svc.BuildImageStream(ctx, testTag, dockerfile)
	if err != nil {
		t.Fatalf("BuildImageStream failed: %v", err)
	}
	defer stream.Close()

	output, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("reading build stream failed: %v", err)
	}

	if len(output) == 0 {
		t.Errorf("expected non-empty output from build stream")
	}

	// Clean up image
	_ = svc.RemoveImage(context.Background(), testTag, true)
}

func TestBuildImageStreamWithContext_InvalidDir(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()
	_, err = svc.BuildImageStreamWithContext(ctx, "test:latest", "FROM scratch", "/nonexistent/path/pdswarm/test")
	if err == nil {
		t.Fatalf("expected error for non-existent context directory, got nil")
	}
}
