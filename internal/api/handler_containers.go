package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/tasks"
)

// HandleListContainers lists containers on the Docker host.
func HandleListContainers(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		all := r.URL.Query().Get("all") == "true" || r.URL.Query().Get("all") == "1"
		containers, err := ds.ListContainers(r.Context(), all)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list containers: "+err.Error())
			return
		}

		if containers == nil {
			containers = []types.Container{}
		}
		respondJSON(w, http.StatusOK, containers)
	}
}

// ContainerInspectResponse embeds types.ContainerJSON and provides separated ImageEnv and UserEnv.
type ContainerInspectResponse struct {
	types.ContainerJSON
	ImageEnv []string `json:"ImageEnv"`
	UserEnv  []string `json:"UserEnv"`
}

// HandleInspectContainer retrieves detailed low-level inspect information for a container.
func HandleInspectContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		containerJSON, err := ds.InspectContainer(r.Context(), id)
		if err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to inspect container: "+err.Error())
			return
		}

		imageEnv := ds.GetImageEnv(r.Context(), containerJSON.Image)
		if len(imageEnv) == 0 && containerJSON.Config != nil && containerJSON.Config.Image != "" {
			imageEnv = ds.GetImageEnv(r.Context(), containerJSON.Config.Image)
		}
		if imageEnv == nil {
			imageEnv = []string{}
		}

		var containerEnv []string
		if containerJSON.Config != nil && len(containerJSON.Config.Env) > 0 {
			containerEnv = containerJSON.Config.Env
		}
		userEnv := docker.FilterUserEnv(containerEnv, imageEnv)
		if userEnv == nil {
			userEnv = []string{}
		}

		resp := ContainerInspectResponse{
			ContainerJSON: containerJSON,
			ImageEnv:      imageEnv,
			UserEnv:       userEnv,
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleCreateContainer creates a new container.
func HandleCreateContainer(ds *docker.Service, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var req docker.CreateContainerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		// Apply default log limits if not explicitly specified
		if req.LogDriver == "" && len(req.LogOpts) == 0 && st != nil {
			driver, _ := st.GetSetting("log_driver")
			maxSize, _ := st.GetSetting("log_max_size")
			maxFile, _ := st.GetSetting("log_max_file")
			compress, _ := st.GetSetting("log_compress")

			if driver == "" {
				driver = "json-file"
			}
			if maxSize == "" {
				maxSize = "10m"
			}
			if maxFile == "" {
				maxFile = "3"
			}

			req.LogDriver = driver
			req.LogOpts = map[string]string{
				"max-size": maxSize,
				"max-file": maxFile,
			}
			if compress == "true" {
				req.LogOpts["compress"] = "true"
			}
		}

		id, err := ds.CreateContainer(r.Context(), req)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to create container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

// HandleStartContainer starts a container.
func HandleStartContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		if err := ds.StartContainer(r.Context(), id); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to start container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container started"})
	}
}

// HandleStopContainer stops a running container.
func HandleStopContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		timeout := 10
		if tStr := r.URL.Query().Get("timeout"); tStr != "" {
			if t, err := strconv.Atoi(tStr); err == nil {
				timeout = t
			}
		}

		if err := ds.StopContainer(r.Context(), id, timeout); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to stop container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container stopped"})
	}
}

// HandleRestartContainer restarts a container.
func HandleRestartContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		timeout := 10
		if tStr := r.URL.Query().Get("timeout"); tStr != "" {
			if t, err := strconv.Atoi(tStr); err == nil {
				timeout = t
			}
		}

		if err := ds.RestartContainer(r.Context(), id, timeout); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to restart container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container restarted"})
	}
}

// HandlePauseContainer pauses processes within a container.
func HandlePauseContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		if err := ds.PauseContainer(r.Context(), id); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to pause container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container paused"})
	}
}

// HandleUnpauseContainer unpauses processes within a paused container.
func HandleUnpauseContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		if err := ds.UnpauseContainer(r.Context(), id); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to unpause container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container unpaused"})
	}
}

// HandleKillContainer sends a signal to kill a container.
func HandleKillContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		signal := r.URL.Query().Get("signal")
		if signal == "" {
			signal = "SIGKILL"
		}

		if err := ds.KillContainer(r.Context(), id, signal); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to kill container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container killed"})
	}
}

// HandleDeleteContainer removes a container.
func HandleDeleteContainer(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"
		removeVolumes := r.URL.Query().Get("v") == "true" || r.URL.Query().Get("remove_volumes") == "true"

		if err := ds.RemoveContainer(r.Context(), id, force, removeVolumes); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to remove container: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "container removed"})
	}
}

// HandlePruneContainers removes all stopped containers.
func HandlePruneContainers(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		report, err := ds.PruneContainers(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to prune containers: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, report)
	}
}

// HandleUpdateContainerImage pulls the latest image for a container and recreates it with identical configuration.
func HandleUpdateContainerImage(ds *docker.Service, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		inspectData, err := ds.InspectContainer(r.Context(), id)
		if err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "container not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to inspect container: "+err.Error())
			return
		}

		cleanName := strings.TrimPrefix(inspectData.Name, "/")
		targetImage := inspectData.Config.Image
		if targetImage == "" {
			targetImage = inspectData.Image
		}
		if targetImage == "" {
			respondJSONError(w, http.StatusBadRequest, "container image is required")
			return
		}

		// Find auth if private registry credentials match
		var authConfig string
		if st != nil {
			if regs, err := st.ListRegistries(); err == nil {
				for _, reg := range regs {
					if reg.URL != "" && strings.Contains(targetImage, strings.TrimPrefix(strings.TrimPrefix(reg.URL, "https://"), "http://")) {
						authConfig = docker.EncodeRegistryAuth(reg.Username, reg.Password, reg.URL)
						break
					}
				}
			}
		}

		req := docker.ContainerToCreateRequest(inspectData)
		wasRunning := inspectData.State != nil && inspectData.State.Running

		task := tasks.DefaultManager().Start(
			tasks.TypeContainerUpdate,
			fmt.Sprintf("Update container %s (%s)", cleanName, targetImage),
			cleanName,
			func(ctx context.Context, t *tasks.Task) error {
				t.SetProgress(15, "Pulling latest image: "+targetImage)
				t.AppendLog("Pulling latest image from registry: " + targetImage)

				stream, err := ds.PullImageStream(ctx, targetImage, authConfig)
				if err != nil {
					return fmt.Errorf("failed to pull image %s: %w", targetImage, err)
				}
				defer stream.Close()

				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					line := scanner.Text()
					t.AppendLog(line)
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("failed to read image pull stream: %w", err)
				}
				t.SetProgress(60, "Stopping existing container...")

				t.AppendLog("Stopping container " + cleanName)
				_ = ds.StopContainer(ctx, id, 10)
				t.SetProgress(75, "Removing old container...")

				t.AppendLog("Removing old container " + cleanName)
				if err := ds.RemoveContainer(ctx, id, true, false); err != nil {
					return fmt.Errorf("failed to remove old container %s: %w", id, err)
				}
				t.SetProgress(85, "Recreating container with updated image...")

				t.AppendLog(fmt.Sprintf("Creating container %s with image %s", req.Name, req.Image))
				newContainerID, err := ds.CreateContainer(ctx, req)
				if err != nil {
					return fmt.Errorf("failed to recreate container %s: %w", req.Name, err)
				}
				t.SetProgress(95, "Starting container...")

				if wasRunning {
					t.AppendLog("Starting container " + newContainerID)
					if err := ds.StartContainer(ctx, newContainerID); err != nil {
						return fmt.Errorf("failed to start updated container: %w", err)
					}
				}

				t.SetProgress(100, "Completed")
				t.AppendLog(fmt.Sprintf("Successfully updated container %s with image %s", cleanName, targetImage))
				return nil
			},
		)

		respondJSON(w, http.StatusAccepted, map[string]interface{}{
			"task_id": task.ID,
			"status":  "updating",
			"message": "container update initiated in background",
		})
	}
}
