package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/docker"
)

func TestWebSocketMissingID(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	r := chi.NewRouter()
	r.Get("/ws/exec", api.HandleContainerExecWS(svc))
	r.Get("/ws/logs", api.HandleContainerLogsWS(svc))
	r.Get("/ws/stats", api.HandleContainerStatsWS(svc))

	server := httptest.NewServer(r)
	defer server.Close()

	client := server.Client()

	for _, endpoint := range []string{"/ws/exec", "/ws/logs", "/ws/stats"} {
		resp, err := client.Get(server.URL + endpoint)
		if err != nil {
			t.Fatalf("GET %s failed: %v", endpoint, err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for %s without ID, got %d", endpoint, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestWebSocketNonExistentContainer(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/ws/containers/{id}/stats", api.HandleContainerStatsWS(svc))
	r.Get("/ws/containers/{id}/logs", api.HandleContainerLogsWS(svc))
	r.Get("/ws/containers/{id}/exec", api.HandleContainerExecWS(svc))

	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Stats with nonexistent ID
	conn, _, err := websocket.Dial(ctx, wsURL+"/ws/containers/nonexistent-container-id/stats", nil)
	if err == nil {
		defer conn.CloseNow()
		_, data, readErr := conn.Read(ctx)
		if readErr == nil {
			t.Logf("stats read error response: %s", string(data))
		}
	}

	// Logs with nonexistent ID
	connLogs, _, err := websocket.Dial(ctx, wsURL+"/ws/containers/nonexistent-container-id/logs", nil)
	if err == nil {
		defer connLogs.CloseNow()
		_, data, readErr := connLogs.Read(ctx)
		if readErr == nil {
			t.Logf("logs read error response: %s", string(data))
		}
	}

	// Exec with nonexistent ID
	connExec, _, err := websocket.Dial(ctx, wsURL+"/ws/containers/nonexistent-container-id/exec", nil)
	if err == nil {
		defer connExec.CloseNow()
		_, data, readErr := connExec.Read(ctx)
		if readErr == nil {
			t.Logf("exec read error response: %s", string(data))
		}
	}
}

func TestWebSocketLiveContainer(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}

	containers, err := svc.ListContainers(ctx, false)
	if err != nil || len(containers) == 0 {
		t.Skip("No running containers found for live websocket test")
	}

	targetContainer := containers[0].ID
	t.Logf("Testing with container %s (%s)", targetContainer[:12], containers[0].Image)

	r := chi.NewRouter()
	r.Get("/ws/containers/{id}/stats", api.HandleContainerStatsWS(svc))
	r.Get("/ws/containers/{id}/logs", api.HandleContainerLogsWS(svc))
	r.Get("/ws/containers/{id}/exec", api.HandleContainerExecWS(svc))

	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	t.Run("StatsWebSocket", func(t *testing.T) {
		subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)
		defer subCancel()

		conn, _, err := websocket.Dial(subCtx, wsURL+"/ws/containers/"+targetContainer+"/stats", nil)
		if err != nil {
			t.Fatalf("failed to dial stats WS: %v", err)
		}
		defer conn.CloseNow()

		_, data, err := conn.Read(subCtx)
		if err != nil {
			t.Fatalf("failed to read from stats WS: %v", err)
		}

		var payload api.ContainerStatsPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("failed to unmarshal stats JSON: %v, raw: %s", err, string(data))
		}

		t.Logf("Received stats payload: CPU=%.2f%%, Mem=%.2fMB / %.2fMB (%.1f%%), NetRx=%d, NetTx=%d",
			payload.CPUPercent, payload.MemoryMB, payload.MemoryLimitMB, payload.MemoryPercent, payload.NetworkRx, payload.NetworkTx)
	})

	t.Run("LogsWebSocket", func(t *testing.T) {
		subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)
		defer subCancel()

		conn, _, err := websocket.Dial(subCtx, wsURL+"/ws/containers/"+targetContainer+"/logs?tail=10", nil)
		if err != nil {
			t.Fatalf("failed to dial logs WS: %v", err)
		}
		defer conn.CloseNow()

		_, data, err := conn.Read(subCtx)
		if err != nil {
			t.Logf("read logs returned: %v (container may have no logs yet)", err)
		} else {
			t.Logf("Received log chunk: %s", string(data))
		}
	})

	t.Run("ExecWebSocketAndResize", func(t *testing.T) {
		subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)
		defer subCancel()

		conn, _, err := websocket.Dial(subCtx, wsURL+"/ws/containers/"+targetContainer+"/exec", nil)
		if err != nil {
			t.Fatalf("failed to dial exec WS: %v", err)
		}
		defer conn.CloseNow()

		// Send resize control frame
		resizeMsg := api.WSResizeMessage{
			Type: "resize",
			Cols: 100,
			Rows: 30,
		}
		resizeBytes, _ := json.Marshal(resizeMsg)
		if err := conn.Write(subCtx, websocket.MessageText, resizeBytes); err != nil {
			t.Fatalf("failed to send resize frame: %v", err)
		}

		// Send echo command into stdin
		echoCmd := []byte("echo __PDSWARM_PTY_OK__\n")
		if err := conn.Write(subCtx, websocket.MessageText, echoCmd); err != nil {
			t.Fatalf("failed to send stdin keystrokes: %v", err)
		}

		// Read output until we see our marker or timeout
		buf := make([]byte, 4096)
		foundMarker := false
		for i := 0; i < 5; i++ {
			_, data, err := conn.Read(subCtx)
			if err != nil {
				break
			}
			buf = append(buf, data...)
			if strings.Contains(string(buf), "__PDSWARM_PTY_OK__") {
				foundMarker = true
				break
			}
		}

		if !foundMarker {
			t.Logf("Read buffer without marker: %s", string(buf))
		} else {
			t.Logf("Successfully executed and received PTY echo marker!")
		}
	})
}
