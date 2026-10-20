package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
)

type wsLogWriter struct {
	conn *websocket.Conn
	ctx  context.Context
	mu   sync.Mutex
}

func (w *wsLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	clean := p
	if !utf8.Valid(p) {
		clean = []byte(strings.ToValidUTF8(string(p), ""))
	}
	if err := w.conn.Write(w.ctx, websocket.MessageText, clean); err != nil {
		return 0, err
	}
	return len(p), nil
}

// HandleContainerLogsWS streams real-time container stdout/stderr logs over WebSockets.
func HandleContainerLogsWS(s *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s = dockerFromContext(r.Context(), s)
		containerID := chi.URLParam(r, "id")
		if containerID == "" {
			containerID = r.URL.Query().Get("id")
		}
		if containerID == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusInternalError, "closing connection")

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		tail := r.URL.Query().Get("tail")
		if tail == "" {
			tail = "200"
		}
		since := r.URL.Query().Get("since")
		until := r.URL.Query().Get("until")
		timestamps := true
		if r.URL.Query().Get("timestamps") == "false" {
			timestamps = false
		}
		follow := true
		if r.URL.Query().Get("follow") == "false" {
			follow = false
		}

		opts := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     follow,
			Timestamps: timestamps,
			Tail:       tail,
			Since:      since,
			Until:      until,
		}

		// Inspect container to check if TTY is enabled
		inspect, err := s.RawClient().ContainerInspect(ctx, containerID)
		isTTY := false
		if err == nil && inspect.Config != nil {
			isTTY = inspect.Config.Tty
		}

		logsReader, err := s.RawClient().ContainerLogs(ctx, containerID, opts)
		if err != nil {
			_ = c.Write(ctx, websocket.MessageText, []byte("Failed to get logs: "+err.Error()+"\r\n"))
			c.Close(websocket.StatusInternalError, "failed to get logs")
			return
		}
		defer logsReader.Close()

		go func() {
			<-ctx.Done()
			_ = logsReader.Close()
		}()

		// Read client messages to detect closure/disconnect
		go func() {
			for {
				_, _, err := c.Read(ctx)
				if err != nil {
					cancel()
					_ = logsReader.Close()
					return
				}
			}
		}()

		writer := &wsLogWriter{
			conn: c,
			ctx:  ctx,
		}

		if isTTY {
			_, _ = io.Copy(writer, logsReader)
		} else {
			_, _ = stdcopy.StdCopy(writer, writer, logsReader)
		}

		c.Close(websocket.StatusNormalClosure, "logs stream finished")
	}
}
