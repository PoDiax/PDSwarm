package docker_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func TestDockerServiceRegistry_SetGetRemove(t *testing.T) {
	reg := docker.NewDockerServiceRegistry()
	defer reg.Close()

	// Initially empty: Get(id) and GetByID(id) return error
	_, err := reg.Get("env-1")
	if err == nil {
		t.Fatalf("expected error getting non-existent service via Get(id), got nil")
	}
	_, err = reg.GetByID("env-1")
	if err == nil {
		t.Fatalf("expected error getting non-existent service via GetByID(id), got nil")
	}

	// Create dummy service or real local service
	localSvc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create local service: %v", err)
	}

	reg.Set("env-1", localSvc)

	got, err := reg.Get("env-1")
	if err != nil {
		t.Fatalf("failed to get service via Get(id): %v", err)
	}
	if got != localSvc {
		t.Fatalf("expected got to equal localSvc")
	}

	gotByID, err := reg.GetByID("env-1")
	if err != nil || gotByID != localSvc {
		t.Fatalf("failed to get service via GetByID(id): %v", err)
	}

	// Remove service
	reg.Remove("env-1")
	_, err = reg.Get("env-1")
	if err == nil {
		t.Fatalf("expected error getting removed service, got nil")
	}
}

func TestDockerServiceRegistry_GetWithPingValidation(t *testing.T) {
	reg := docker.NewRegistry()
	defer reg.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	localEnv := &store.Environment{
		ID:   "local",
		Name: "Local Docker",
		Type: "local",
		URL:  "",
	}

	// 1. GetByEnv should connect and ping successfully for local daemon
	svc, err := reg.GetByEnv(ctx, localEnv)
	if err != nil {
		t.Fatalf("failed to get local service: %v", err)
	}
	if svc == nil {
		t.Fatalf("expected non-nil service")
	}

	// Verify it can now be retrieved via Get(id)
	cachedFromID, err := reg.Get(localEnv.ID)
	if err != nil || cachedFromID != svc {
		t.Fatalf("expected Get(id) to return cached instance: %v", err)
	}

	// 2. Second GetByEnv should return cached instance immediately
	cachedSvc, err := reg.GetByEnv(ctx, localEnv)
	if err != nil {
		t.Fatalf("failed to get cached service: %v", err)
	}
	if cachedSvc != svc {
		t.Fatalf("expected cached service to match original service instance")
	}

	// 3. Unreachable host should fail ping validation
	badEnv := &store.Environment{
		ID:   "bad-host",
		Name: "Unreachable",
		Type: "tcp",
		URL:  "tcp://127.0.0.1:54321", // unused port
	}

	quickCtx, quickCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer quickCancel()

	_, err = reg.GetByEnv(quickCtx, badEnv)
	if err == nil {
		t.Fatalf("expected ping validation to fail for bad host, got nil")
	}
}

func TestNewServiceFromEnvironment_Types(t *testing.T) {
	// Nil env
	_, err := docker.NewServiceFromEnvironment(nil)
	if err == nil {
		t.Fatalf("expected error for nil environment")
	}

	// Local env
	localEnv := &store.Environment{
		ID:   "local",
		Type: "local",
		URL:  "/var/run/docker.sock",
	}
	svc, err := docker.NewServiceFromEnvironment(localEnv)
	if err != nil {
		t.Fatalf("failed to create local service: %v", err)
	}
	_ = svc.Close()

	// TCP env without scheme (should auto-prepend tcp://)
	tcpEnv := &store.Environment{
		ID:   "tcp-env",
		Type: "tcp",
		URL:  "127.0.0.1:2375",
	}
	svc, err = docker.NewServiceFromEnvironment(tcpEnv)
	if err != nil {
		t.Fatalf("failed to create tcp service: %v", err)
	}
	_ = svc.Close()

	// Empty TCP url
	_, err = docker.NewServiceFromEnvironment(&store.Environment{Type: "tcp", URL: ""})
	if err == nil {
		t.Fatalf("expected error for empty TCP url")
	}

	// TLS env with PEM string data
	fakeCA := "-----BEGIN CERTIFICATE-----\nMIIC...fakeCA...\n-----END CERTIFICATE-----\n"
	fakeCert := "-----BEGIN CERTIFICATE-----\nMIIC...fakeCert...\n-----END CERTIFICATE-----\n"
	fakeKey := "-----BEGIN RSA PRIVATE KEY-----\nMIIE...fakeKey...\n-----END RSA PRIVATE KEY-----\n"

	tlsEnv := &store.Environment{
		ID:      "tls-env",
		Type:    "tls",
		URL:     "tcp://127.0.0.1:2376",
		TLSCA:   fakeCA,
		TLSCert: fakeCert,
		TLSKey:  fakeKey,
	}

	// With invalid PEM bytes, NewServiceFromEnvironment will fail in tlsconfig, confirming the paths were passed
	_, err = docker.NewServiceFromEnvironment(tlsEnv)
	if err == nil {
		t.Fatalf("expected error initializing TLS with dummy PEM data")
	}
	if !strings.Contains(err.Error(), "failed to process TLS") && !strings.Contains(err.Error(), "failed to initialize docker client") && !strings.Contains(err.Error(), "failed to create tls config") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Unsupported type
	_, err = docker.NewServiceFromEnvironment(&store.Environment{Type: "invalid-type"})
	if err == nil {
		t.Fatalf("expected error for invalid type")
	}
}

func TestDockerServiceRegistry_Close(t *testing.T) {
	reg := docker.NewDockerServiceRegistry()

	svc1, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service 1: %v", err)
	}
	svc2, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create service 2: %v", err)
	}

	reg.Set("env-1", svc1)
	reg.Set("env-2", svc2)

	if err := reg.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Registry should now be empty
	if _, err := reg.Get("env-1"); err == nil {
		t.Fatalf("expected env-1 to be gone after Close")
	}
	if _, err := reg.Get("env-2"); err == nil {
		t.Fatalf("expected env-2 to be gone after Close")
	}
}
