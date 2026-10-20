package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
)

// WSResizeMessage represents an incoming window resize control frame from xterm.js.
type WSResizeMessage struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

// HandleContainerExecWS bridges a browser xterm.js terminal with a container PTY over WebSockets.
func HandleContainerExecWS(s *docker.Service) http.HandlerFunc {
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

		// Determine shell command: check custom query param, then fallback from /bin/bash to /bin/sh
		cmd := []string{"/bin/bash"}
		if customCmd := r.URL.Query().Get("cmd"); customCmd != "" {
			cmd = []string{customCmd}
		} else {
			if _, err := s.RawClient().ContainerStatPath(ctx, containerID, "/bin/bash"); err != nil {
				cmd = []string{"/bin/sh"}
			}
		}

		execConfig := container.ExecOptions{
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Tty:          true,
			Cmd:          cmd,
			Env:          []string{"TERM=xterm-256color"},
		}

		execResp, err := s.RawClient().ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			_ = c.Write(ctx, websocket.MessageText, []byte("Failed to create exec process: "+err.Error()+"\r\n"))
			c.Close(websocket.StatusInternalError, "failed to create exec")
			return
		}

		attachResp, err := s.RawClient().ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
			Tty: true,
		})
		if err != nil {
			_ = c.Write(ctx, websocket.MessageText, []byte("Failed to attach to exec process: "+err.Error()+"\r\n"))
			c.Close(websocket.StatusInternalError, "failed to attach to exec")
			return
		}
		defer attachResp.Close()

		// Apply initial resize if supplied in query params
		if colsStr, rowsStr := r.URL.Query().Get("cols"), r.URL.Query().Get("rows"); colsStr != "" && rowsStr != "" {
			cols, errC := strconv.ParseUint(colsStr, 10, 32)
			rows, errR := strconv.ParseUint(rowsStr, 10, 32)
			if errC == nil && errR == nil && cols > 0 && rows > 0 {
				_ = s.RawClient().ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
					Height: uint(rows),
					Width:  uint(cols),
				})
			}
		}

		// Goroutine 1: Container PTY stdout -> WebSocket
		go func() {
			defer cancel()
			buf := make([]byte, 4096)
			for {
				n, err := attachResp.Reader.Read(buf)
				if n > 0 {
					msgType := websocket.MessageText
					if !utf8.Valid(buf[:n]) {
						msgType = websocket.MessageBinary
					}
					if wErr := c.Write(ctx, msgType, buf[:n]); wErr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()

		// Goroutine 2 (Main loop): WebSocket -> Container PTY stdin & Resize messages
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				break
			}

			// Intercept JSON resize control frame
			var resizeMsg WSResizeMessage
			if json.Unmarshal(data, &resizeMsg) == nil && resizeMsg.Type == "resize" {
				if resizeMsg.Cols > 0 && resizeMsg.Rows > 0 {
					_ = s.RawClient().ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
						Height: resizeMsg.Rows,
						Width:  resizeMsg.Cols,
					})
				}
				continue
			}

			// Forward raw input keystrokes to container PTY
			if _, err := attachResp.Conn.Write(data); err != nil {
				break
			}
		}

		c.Close(websocket.StatusNormalClosure, "terminal exited")
	}
}
