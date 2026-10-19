package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/tasks"
)

type PullImageRequest struct {
	Image      string `json:"image"`
	FromImage  string `json:"fromImage"`
	Ref        string `json:"ref"`
	RegistryID string `json:"registry_id"`
	Async      bool   `json:"async"`
}

type CheckImageUpdateRequest struct {
	Image      string `json:"image"`
	RegistryID string `json:"registry_id,omitempty"`
}

type CheckImageUpdatesBatchRequest struct {
	Images     []string `json:"images"`
	RegistryID string   `json:"registry_id,omitempty"`
}

type CheckImageUpdatesBatchResponse struct {
	Results map[string]*docker.ImageUpdateInfo `json:"results"`
}

type dockerPullLine struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

// extractImageDomain extracts the registry host/domain from a Docker image reference.
func extractImageDomain(ref string) string {
	ref = strings.TrimSpace(ref)
	parts := strings.Split(ref, "/")
	if len(parts) <= 1 {
		return "docker.io"
	}
	first := parts[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first
	}
	return "docker.io"
}

// HandleListImages returns all images on the Docker host.
func HandleListImages(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		images, err := ds.ListImages(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list images: "+err.Error())
			return
		}

		if images == nil {
			images = []image.Summary{}
		}
		respondJSON(w, http.StatusOK, images)
	}
}

// HandleCheckImageUpdate checks whether a local image has an update available on its remote registry.
func HandleCheckImageUpdate(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		ref := r.URL.Query().Get("image")
		if ref == "" {
			ref = chi.URLParam(r, "id")
		}
		registryID := r.URL.Query().Get("registry_id")

		if r.Body != nil && r.ContentLength > 0 {
			var req CheckImageUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				if req.Image != "" {
					ref = req.Image
				}
				if req.RegistryID != "" {
					registryID = req.RegistryID
				}
			}
		}

		ref = strings.TrimSpace(ref)
		if ref == "" {
			respondJSONError(w, http.StatusBadRequest, "image reference is required")
			return
		}

		var authConfig string
		if st != nil {
			var reg *store.Registry
			if registryID != "" {
				reg, _ = st.GetRegistry(registryID)
			} else {
				domain := extractImageDomain(ref)
				if domain != "" {
					reg, _ = st.GetRegistryByURL(domain)
				}
			}
			if reg != nil && (reg.Username != "" || reg.Password != "") {
				authConfig = docker.EncodeRegistryAuth(reg.Username, reg.Password, reg.URL)
			}
		}

		checkCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		info, err := ds.CheckImageUpdate(checkCtx, ref, authConfig)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to check image update: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, info)
	}
}

// HandleCheckImageUpdatesBatch concurrently checks updates for multiple Docker images.
func HandleCheckImageUpdatesBatch(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetDS := dockerFromContext(r.Context(), ds)
		if targetDS == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var req CheckImageUpdatesBatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
			return
		}

		// Filter and deduplicate images
		seen := make(map[string]bool)
		var uniqueImages []string
		for _, img := range req.Images {
			trimmed := strings.TrimSpace(img)
			if trimmed == "" || strings.HasPrefix(trimmed, "<none>") || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			uniqueImages = append(uniqueImages, trimmed)
		}

		results := make(map[string]*docker.ImageUpdateInfo)
		if len(uniqueImages) == 0 {
			respondJSON(w, http.StatusOK, CheckImageUpdatesBatchResponse{Results: results})
			return
		}

		// Worker pool for concurrency (up to 8 concurrent checks)
		maxWorkers := 8
		if len(uniqueImages) < maxWorkers {
			maxWorkers = len(uniqueImages)
		}

		jobs := make(chan string, len(uniqueImages))
		for _, img := range uniqueImages {
			jobs <- img
		}
		close(jobs)

		var mu sync.Mutex
		var wg sync.WaitGroup

		for wIdx := 0; wIdx < maxWorkers; wIdx++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ref := range jobs {
					var authConfig string
					if st != nil {
						var reg *store.Registry
						if req.RegistryID != "" {
							reg, _ = st.GetRegistry(req.RegistryID)
						} else {
							domain := extractImageDomain(ref)
							if domain != "" {
								reg, _ = st.GetRegistryByURL(domain)
							}
						}
						if reg != nil && (reg.Username != "" || reg.Password != "") {
							authConfig = docker.EncodeRegistryAuth(reg.Username, reg.Password, reg.URL)
						}
					}

					checkCtx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
					info, err := targetDS.CheckImageUpdate(checkCtx, ref, authConfig)
					cancel()

					if err != nil {
						info = &docker.ImageUpdateInfo{
							ImageRef: ref,
							Status:   "unknown",
							Error:    err.Error(),
							Message:  "Check failed: " + err.Error(),
						}
					}

					mu.Lock()
					results[ref] = info
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		respondJSON(w, http.StatusOK, CheckImageUpdatesBatchResponse{Results: results})
	}
}

// HandlePullImage pulls a Docker image from a registry in a detached background task.
// If stream=true is provided, it streams progress lines live to the client while still running safely in the background.
func HandlePullImage(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetDS := dockerFromContext(r.Context(), ds)
		if targetDS == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var ref string
		var registryID string
		var asyncRequested bool
		if r.Body != nil && r.ContentLength > 0 {
			var req PullImageRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				ref = req.Image
				if ref == "" {
					ref = req.FromImage
				}
				if ref == "" {
					ref = req.Ref
				}
				registryID = req.RegistryID
				asyncRequested = req.Async
			}
		}

		if ref == "" {
			ref = r.URL.Query().Get("fromImage")
		}
		if ref == "" {
			ref = r.URL.Query().Get("image")
		}
		if registryID == "" {
			registryID = r.URL.Query().Get("registry_id")
			if registryID == "" {
				registryID = r.URL.Query().Get("registryId")
			}
		}
		if r.URL.Query().Get("async") == "true" {
			asyncRequested = true
		}

		ref = strings.TrimSpace(ref)
		if ref == "" {
			respondJSONError(w, http.StatusBadRequest, "image reference is required")
			return
		}

		var authConfig string
		if st != nil {
			var reg *store.Registry
			if registryID != "" {
				reg, _ = st.GetRegistry(registryID)
			} else {
				domain := extractImageDomain(ref)
				if domain != "" {
					reg, _ = st.GetRegistryByURL(domain)
				}
			}
			if reg != nil && (reg.Username != "" || reg.Password != "") {
				authConfig = docker.EncodeRegistryAuth(reg.Username, reg.Password, reg.URL)
			}
		}

		isStreaming := r.URL.Query().Get("stream") == "true"
		streamCh := make(chan []byte, 100)

		// Start background task in manager
		task := tasks.DefaultManager().Start(
			tasks.TypeImagePull,
			"Pulling "+ref,
			ref,
			func(ctx context.Context, t *tasks.Task) error {
				defer close(streamCh)

				t.AppendLog(fmt.Sprintf("Contacting registry for %s...", ref))
				rc, err := targetDS.PullImageStream(ctx, ref, authConfig)
				if err != nil {
					return err
				}
				defer rc.Close()

				scanner := bufio.NewScanner(rc)
				layers := make(map[string]*dockerPullLine)

				for scanner.Scan() {
					lineBytes := scanner.Bytes()
					if len(lineBytes) == 0 {
						continue
					}

					// Non-blocking forward to active HTTP stream if connected
					select {
					case streamCh <- append([]byte(nil), lineBytes...):
					default:
					}

					var p dockerPullLine
					if err := json.Unmarshal(lineBytes, &p); err == nil {
						if p.Error != "" {
							return errors.New(p.Error)
						}
						if p.ID != "" {
							layers[p.ID] = &p
						}

						// Calculate aggregate progress percentage
						var curSum, totSum int64
						for _, l := range layers {
							if l.ProgressDetail.Total > 0 {
								curSum += l.ProgressDetail.Current
								totSum += l.ProgressDetail.Total
							}
						}

						if totSum > 0 {
							pct := int((curSum * 100) / totSum)
							t.SetProgress(pct, fmt.Sprintf("%s (%d/%d MB)", p.Status, curSum/1024/1024, totSum/1024/1024))
						} else if p.Status != "" {
							t.SetProgress(t.Progress, p.Status)
						}

						// Format clean log message
						logMsg := p.Status
						if p.ID != "" {
							logMsg = p.ID + ": " + logMsg
						}
						if p.Progress != "" {
							logMsg += " " + p.Progress
						}
						t.AppendLog(logMsg)
					}
				}

				if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
					return err
				}

				t.SetProgress(100, "Successfully pulled "+ref)
				t.AppendLog("Image pull completed successfully.")
				return nil
			},
		)

		// 1. If client explicitly requested async (or doesn't want stream), return task descriptor
		if asyncRequested {
			respondJSON(w, http.StatusAccepted, map[string]interface{}{
				"task_id": task.ID,
				"status":  task.Status,
				"title":   task.Title,
				"image":   ref,
			})
			return
		}

		// 2. If client wants live streaming, pipe streamCh to response until closed or client disconnects
		if isStreaming {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Task-ID", task.ID)
			w.WriteHeader(http.StatusOK)

			flusher, canFlush := w.(http.Flusher)

			for {
				select {
				case <-r.Context().Done():
					// Client disconnected/closed browser, but task continues in background!
					return
				case line, ok := <-streamCh:
					if !ok {
						return
					}
					_, _ = w.Write(line)
					_, _ = w.Write([]byte("\n"))
					if canFlush {
						flusher.Flush()
					}
				}
			}
		}

		// 3. Otherwise wait up to a few seconds or return task response
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"task_id": task.ID,
			"message": "image pull initiated",
			"image":   ref,
		})
	}
}

// HandleDeleteImage removes an image by ID or reference name.
func HandleDeleteImage(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "image id is required")
			return
		}

		force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"

		if err := ds.RemoveImage(r.Context(), id, force); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "image not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to remove image: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "image removed"})
	}
}

// HandlePruneImages deletes unused images.
func HandlePruneImages(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		report, err := ds.PruneImages(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to prune images: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, report)
	}
}
