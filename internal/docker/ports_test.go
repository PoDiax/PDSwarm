package docker

import (
	"context"
	"fmt"
	"net"
	"testing"
)

func TestPortConflictDetection_OSListener(t *testing.T) {
	svc := &Service{}

	// Bind a local TCP listener on random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on local port: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Test IsPortInUse on this bound port
	inUse, reason := svc.IsPortInUse(port, "tcp", nil)
	if !inUse {
		t.Errorf("expected port %d to be in use, got false", port)
	}
	if reason == "" {
		t.Errorf("expected reason for port in use")
	}

	// Test IsPortInUse on next port (or free port)
	freePort := svc.FindNextAvailablePort(port+1, "tcp", nil)
	if freePort <= port {
		t.Errorf("expected suggested port > %d, got %d", port, freePort)
	}

	freeInUse, _ := svc.IsPortInUse(freePort, "tcp", nil)
	if freeInUse {
		t.Errorf("expected suggested port %d to be free, got in use", freePort)
	}
}

func TestPortConflictDetection_ContainerAllocations(t *testing.T) {
	svc := &Service{}

	allocs := []PortAllocation{
		{
			HostIP:        "0.0.0.0",
			HostPort:      8080,
			ContainerPort: 80,
			Protocol:      "tcp",
			ContainerID:   "c123456",
			ContainerName: "test-nginx",
			State:         "running",
		},
	}

	inUse, reason := svc.IsPortInUse(8080, "tcp", allocs)
	if !inUse {
		t.Errorf("expected port 8080 to be reported in use")
	}
	if reason != "container test-nginx (running)" {
		t.Errorf("unexpected conflict reason: %s", reason)
	}

	// Check conflict check helper with a bound listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on local port: %v", err)
	}
	defer ln.Close()
	boundPort := ln.Addr().(*net.TCPAddr).Port

	res, err := svc.CheckPortConflicts(context.Background(), []string{fmt.Sprintf("%d:80/tcp", boundPort)})
	if err != nil {
		t.Fatalf("CheckPortConflicts failed: %v", err)
	}
	if !res.HasConflict {
		t.Errorf("expected HasConflict=true")
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(res.Conflicts))
	}
	if res.Conflicts[0].SuggestedPort == boundPort {
		t.Errorf("expected suggested port to differ from %d, got %d", boundPort, res.Conflicts[0].SuggestedPort)
	}

	// Verify CheckPortConflicts handles ${PORT:-3000} syntax without error
	varRes, err := svc.CheckPortConflicts(context.Background(), []string{"${PORT:-3000}"})
	if err != nil {
		t.Fatalf("CheckPortConflicts with variable failed: %v", err)
	}
	if len(varRes.Conflicts) != 1 {
		t.Fatalf("expected 1 port conflict entry for variable port, got %d", len(varRes.Conflicts))
	}
	if varRes.Conflicts[0].HostPort != 3000 || varRes.Conflicts[0].ContainerPort != 3000 {
		t.Errorf("expected resolved port 3000:3000, got %d:%d", varRes.Conflicts[0].HostPort, varRes.Conflicts[0].ContainerPort)
	}
}
