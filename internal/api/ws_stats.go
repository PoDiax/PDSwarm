package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
)

// HandleContainerStatsWS streams real-time container metrics over WebSockets.
func HandleContainerStatsWS(s *docker.Service) http.HandlerFunc {
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

		// Monitor client disconnect
		go func() {
			for {
				_, _, err := c.Read(ctx)
				if err != nil {
					cancel()
					return
				}
			}
		}()

		statsResp, err := s.RawClient().ContainerStats(ctx, containerID, true)
		if err != nil {
			_ = c.Write(ctx, websocket.MessageText, []byte(`{"error":"`+err.Error()+`"}`))
			c.Close(websocket.StatusInternalError, "failed to get container stats")
			return
		}
		defer statsResp.Body.Close()

		go func() {
			<-ctx.Done()
			_ = statsResp.Body.Close()
		}()

		decoder := json.NewDecoder(statsResp.Body)
		for {
			var stats container.StatsResponse
			if err := decoder.Decode(&stats); err != nil {
				break
			}

			payload := CalculateStatsSnapshot(&stats)
			data, err := json.Marshal(payload)
			if err != nil {
				break
			}

			if err := c.Write(ctx, websocket.MessageText, data); err != nil {
				break
			}
		}

		c.Close(websocket.StatusNormalClosure, "stats stream finished")
	}
}
