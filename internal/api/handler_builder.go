package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/podiax/pdswarm/internal/docker"
)

// BuildImageRequest represents the JSON request payload for building a Docker image.
type BuildImageRequest struct {
	Tag        string `json:"tag"`
	Dockerfile string `json:"dockerfile"`
	ContextDir string `json:"context_dir,omitempty"`
}

// HandleBuildImage handles POST /api/images/build and streams Docker build progress.
func HandleBuildImage(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		if r.Body == nil || r.Body == http.NoBody {
			respondJSONError(w, http.StatusBadRequest, "request body is required")
			return
		}

		var req BuildImageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Tag = strings.TrimSpace(req.Tag)
		if req.Tag == "" {
			respondJSONError(w, http.StatusBadRequest, "image tag is required")
			return
		}

		if strings.TrimSpace(req.Dockerfile) == "" {
			respondJSONError(w, http.StatusBadRequest, "dockerfile content is required")
			return
		}

		rc, err := ds.BuildImageStreamWithContext(r.Context(), req.Tag, req.Dockerfile, req.ContextDir)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to start image build: "+err.Error())
			return
		}
		defer rc.Close()

		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		flusher, hasFlusher := w.(http.Flusher)
		if hasFlusher {
			flusher.Flush()
		}

		reader := bufio.NewReader(rc)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				_, _ = w.Write(line)
				if line[len(line)-1] != '\n' {
					_, _ = w.Write([]byte("\n"))
				}
				if hasFlusher {
					flusher.Flush()
				}
			}
			if err != nil {
				break
			}
		}
	}
}
