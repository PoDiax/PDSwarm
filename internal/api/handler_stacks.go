package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/tasks"
)

type CreateStackRequest struct {
	Name        string `json:"name"`
	ComposeYAML string `json:"compose_yaml"`
	Env         string `json:"env,omitempty"`
	Deploy      *bool  `json:"deploy,omitempty"`
	PullImages  bool   `json:"pull_images,omitempty"`
}

type UpdateStackRequest struct {
	ComposeYAML string `json:"compose_yaml"`
	Env         string `json:"env,omitempty"`
	Deploy      *bool  `json:"deploy,omitempty"`
	PullImages  bool   `json:"pull_images,omitempty"`
}

type AdoptStackRequest struct {
	ComposeYAML string `json:"compose_yaml,omitempty"`
	Env         string `json:"env,omitempty"`
	WorkingDir  string `json:"working_dir,omitempty"`
}

func extractComposeDetails(ctx context.Context, ds *docker.Service, stackName string, containers []docker.ContainerSummary) (yamlContent string, envContent string, configFilePath string, workingDir string) {
	if len(containers) == 0 {
		return "", "", "", ""
	}
	if ds != nil {
		return ds.ExtractOrSynthesizeComposeDetails(ctx, stackName, containers)
	}
	return extractComposeDetailsFast(containers)
}

// extractComposeDetailsFast reads compose YAML and env files directly from the host filesystem
// using container labels without making any additional Docker API calls or container inspections.
func extractComposeDetailsFast(containers []docker.ContainerSummary) (yamlContent string, envContent string, configFilePath string, workingDir string) {
	if len(containers) == 0 {
		return "", "", "", ""
	}
	c := containers[0]
	workingDir = c.Labels["com.docker.compose.project.working_dir"]
	configFile := c.Labels["com.docker.compose.project.config_files"]
	if configFile != "" {
		files := strings.Split(configFile, ",")
		for _, f := range files {
			trimmed := strings.TrimSpace(f)
			if trimmed != "" {
				configFilePath = trimmed
				if data, err := os.ReadFile(trimmed); err == nil && len(data) > 0 {
					yamlContent = string(data)
					if workingDir == "" {
						workingDir = filepath.Dir(trimmed)
					}
					break
				}
			}
		}
	}
	if workingDir != "" {
		for _, name := range []string{"stack.env", ".env"} {
			if data, err := os.ReadFile(filepath.Join(workingDir, name)); err == nil && len(data) > 0 {
				envContent = string(data)
				break
			}
		}
	}
	return yamlContent, envContent, configFilePath, workingDir
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondJSONError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// HandleListStacks lists all stacks saved in store, and discovers external compose stacks.
func HandleListStacks(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		stacks, err := st.ListStacks()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if stacks == nil {
			stacks = []*store.Stack{}
		}

		if ds != nil {
			existing := make(map[string]bool)
			for _, s := range stacks {
				existing[s.Name] = true
			}

			containers, err := ds.ListContainers(r.Context(), true)
			if err == nil {
				discovered := make(map[string][]docker.ContainerSummary)
				for _, c := range containers {
					proj := c.Labels[docker.ComposeProjectLabel]
					if proj != "" && !existing[proj] {
						discovered[proj] = append(discovered[proj], c)
					}
				}

				for proj, projContainers := range discovered {
					var yamlContent, envContent, configFile, workingDir string
					if cache := ds.GetStackDetailCache(); cache != nil {
						if details, found := cache.Get(proj); found {
							yamlContent = details.ComposeYAML
							envContent = details.Env
							configFile = details.ConfigFile
							workingDir = details.WorkingDir
						}
					}
					if yamlContent == "" {
						yamlContent, envContent, configFile, workingDir = extractComposeDetailsFast(projContainers)
						if yamlContent == "" && ds != nil {
							ds.EnqueueStackEnrichment(proj, projContainers)
						}
					}
					createdAt := time.Unix(projContainers[0].Created, 0)
					if projContainers[0].Created == 0 {
						createdAt = time.Now()
					}
					stacks = append(stacks, &store.Stack{
						Name:        proj,
						ComposeYAML: yamlContent,
						Env:         envContent,
						Status:      "running",
						IsExternal:  true,
						ConfigFile:  configFile,
						WorkingDir:  workingDir,
						CreatedAt:   createdAt,
						UpdatedAt:   createdAt,
					})
				}
			}
		}

		respondJSON(w, http.StatusOK, stacks)
	}
}

// HandleGetStack returns the stack details and optionally running containers.
func HandleGetStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "stack name is required")
			return
		}

		stack, err := st.GetStack(name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if ds != nil {
					containers, cErr := ds.GetStackContainers(r.Context(), name)
					if cErr == nil && len(containers) > 0 {
						var yamlContent, envContent, configFile, workingDir string
						if cache := ds.GetStackDetailCache(); cache != nil {
							if details, found := cache.Get(name); found {
								yamlContent = details.ComposeYAML
								envContent = details.Env
								configFile = details.ConfigFile
								workingDir = details.WorkingDir
							}
						}
						if yamlContent == "" {
							yamlContent, envContent, configFile, workingDir = extractComposeDetails(r.Context(), ds, name, containers)
							if cache := ds.GetStackDetailCache(); cache != nil && yamlContent != "" {
								cache.Set(name, docker.CachedStackDetails{
									ComposeYAML: yamlContent,
									Env:         envContent,
									ConfigFile:  configFile,
									WorkingDir:  workingDir,
								})
							}
						}
						createdAt := time.Unix(containers[0].Created, 0)
						if containers[0].Created == 0 {
							createdAt = time.Now()
						}
						stack = &store.Stack{
							Name:        name,
							ComposeYAML: yamlContent,
							Env:         envContent,
							Status:      "running",
							IsExternal:  true,
							ConfigFile:  configFile,
							WorkingDir:  workingDir,
							CreatedAt:   createdAt,
							UpdatedAt:   createdAt,
						}
						resp := struct {
							*store.Stack
							Containers []docker.ContainerSummary `json:"containers"`
						}{
							Stack:      stack,
							Containers: containers,
						}
						respondJSON(w, http.StatusOK, resp)
						return
					}
				}
				respondJSONError(w, http.StatusNotFound, "stack not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if ds != nil {
			containers, err := ds.GetStackContainers(r.Context(), name)
			if err == nil {
				if stack.ComposeYAML == "" && len(containers) > 0 {
					recoveredYAML, recoveredEnv, cfgFile, wDir := extractComposeDetails(r.Context(), ds, name, containers)
					if recoveredYAML != "" {
						stack.ComposeYAML = recoveredYAML
						if stack.ConfigFile == "" {
							stack.ConfigFile = cfgFile
						}
						if stack.WorkingDir == "" {
							stack.WorkingDir = wDir
						}
						if stack.Env == "" && recoveredEnv != "" {
							stack.Env = recoveredEnv
						}
						_ = st.SaveStack(stack)
					}
					if cache := ds.GetStackDetailCache(); cache != nil && recoveredYAML != "" {
						cache.Set(name, docker.CachedStackDetails{
							ComposeYAML: recoveredYAML,
							Env:         recoveredEnv,
							ConfigFile:  cfgFile,
							WorkingDir:  wDir,
						})
					}
				}
				if stack.Env == "" && len(containers) > 0 {
					recoveredEnv := ds.ExtractOrSynthesizeStackEnv(r.Context(), stack.WorkingDir, containers)
					if recoveredEnv != "" {
						stack.Env = recoveredEnv
						_ = st.SaveStack(stack)
					}
				}
				resp := struct {
					*store.Stack
					Containers []docker.ContainerSummary `json:"containers"`
				}{
					Stack:      stack,
					Containers: containers,
				}
				respondJSON(w, http.StatusOK, resp)
				return
			}
		}

		if stack.Env == "" && stack.WorkingDir != "" {
			for _, name := range []string{"stack.env", ".env"} {
				envFile := filepath.Join(stack.WorkingDir, name)
				if envData, err := os.ReadFile(envFile); err == nil && len(envData) > 0 {
					stack.Env = string(envData)
					_ = st.SaveStack(stack)
					break
				}
			}
		}

		respondJSON(w, http.StatusOK, stack)
	}
}

// HandleCreateStack creates and optionally deploys a new stack.
func HandleCreateStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		var req CreateStackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			respondJSONError(w, http.StatusBadRequest, "name is required")
			return
		}

		trimmedYAML := strings.TrimSpace(req.ComposeYAML)
		if trimmedYAML == "" {
			respondJSONError(w, http.StatusBadRequest, "compose_yaml is required")
			return
		}

		envMap := docker.ParseEnvFile(req.Env)
		if _, err := docker.ParseComposeYAMLWithEnv(trimmedYAML, envMap); err != nil {
			respondJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose yaml: %v", err))
			return
		}

		// Check if stack already exists
		existing, err := st.GetStack(req.Name)
		if err == nil && existing != nil {
			respondJSONError(w, http.StatusConflict, fmt.Sprintf("stack %q already exists", req.Name))
			return
		}

		stack := &store.Stack{
			Name:        req.Name,
			ComposeYAML: trimmedYAML,
			Env:         req.Env,
			Status:      "created",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		shouldDeploy := true
		if req.Deploy != nil {
			shouldDeploy = *req.Deploy
		}

		if shouldDeploy && ds != nil {
			stack.Status = "deploying"
			if err := ds.DeployStackWithOptions(r.Context(), stack, nil, docker.DeployOptions{
				PullImages: req.PullImages,
			}); err != nil {
				_ = st.DeleteStack(stack.Name)
				respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to deploy stack: %v", err))
				return
			}
			stack.Status = "running"
		}

		if err := st.SaveStack(stack); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save stack: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, stack)
	}
}

// HandleUpdateStack updates the compose YAML of a stack and optionally redeploys it.
func HandleUpdateStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "stack name is required")
			return
		}

		stack, err := st.GetStack(name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// Only adopt if external containers actually exist for this stack
				if ds != nil {
					containers, cErr := ds.GetStackContainers(r.Context(), name)
					if cErr == nil && len(containers) > 0 {
						_, diskEnv, cfgFile, wDir := extractComposeDetails(r.Context(), ds, name, containers)
						stack = &store.Stack{
							Name:       name,
							Env:        diskEnv,
							ConfigFile: cfgFile,
							WorkingDir: wDir,
							CreatedAt:  time.Now(),
						}
					}
				}
				if stack == nil {
					respondJSONError(w, http.StatusNotFound, "stack not found")
					return
				}
			} else {
				respondJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		var req UpdateStackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		trimmedYAML := strings.TrimSpace(req.ComposeYAML)
		if trimmedYAML == "" {
			respondJSONError(w, http.StatusBadRequest, "compose_yaml is required")
			return
		}

		envMap := docker.ParseEnvFile(req.Env)
		if _, err := docker.ParseComposeYAMLWithEnv(trimmedYAML, envMap); err != nil {
			respondJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose yaml: %v", err))
			return
		}

		stack.ComposeYAML = trimmedYAML
		stack.Env = req.Env
		stack.IsExternal = false // Elevated to fully managed PDSwarm stack
		stack.UpdatedAt = time.Now()

		shouldDeploy := true
		if req.Deploy != nil {
			shouldDeploy = *req.Deploy
		}

		if shouldDeploy && ds != nil {
			stack.Status = "deploying"
			if err := st.SaveStack(stack); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to save stack: "+err.Error())
				return
			}

			taskStack := *stack
			tasks.DefaultManager().Start(
				tasks.TypeStackDeploy,
				fmt.Sprintf("Redeploy stack %s", stack.Name),
				stack.Name,
				func(ctx context.Context, t *tasks.Task) error {
					t.SetProgress(5, fmt.Sprintf("Preparing deployment for %s...", taskStack.Name))
					t.AppendLog(fmt.Sprintf("Starting redeployment for stack %q", taskStack.Name))

					onProgress := func(evt docker.DeployEvent) {
						if evt.Status != "" {
							t.AppendLog(evt.Status)
						}
						if evt.Detail != "" {
							t.AppendLog(evt.Detail)
						}
						switch evt.Step {
						case "init":
							t.SetProgress(5, evt.Status)
						case "network":
							t.SetProgress(15, evt.Status)
						case "pull":
							t.SetProgress(40, evt.Status)
						case "recreate":
							t.SetProgress(70, evt.Status)
						case "start":
							t.SetProgress(85, evt.Status)
						case "complete":
							t.SetProgress(100, evt.Status)
						}
					}

					deployErr := ds.DeployStackWithOptions(ctx, &taskStack, onProgress, docker.DeployOptions{
						PullImages: req.PullImages,
					})
					if deployErr != nil {
						taskStack.Status = "error"
						_ = st.SaveStack(&taskStack)
						t.AppendLog(fmt.Sprintf("Deployment failed: %v", deployErr))
						return deployErr
					}

					taskStack.Status = "running"
					_ = st.SaveStack(&taskStack)
					if cache := ds.GetStackDetailCache(); cache != nil {
						cache.Invalidate(taskStack.Name)
					}
					t.SetProgress(100, "Completed")
					t.AppendLog(fmt.Sprintf("Stack %q redeployed successfully", taskStack.Name))
					return nil
				},
			)
		} else {
			if err := st.SaveStack(stack); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to save stack: "+err.Error())
				return
			}
			if ds != nil {
				if cache := ds.GetStackDetailCache(); cache != nil {
					cache.Invalidate(name)
				}
			}
		}

		respondJSON(w, http.StatusOK, stack)
	}
}

// HandleStopStack stops running containers of a stack.
func HandleStopStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "stack name is required")
			return
		}

		stack, err := st.GetStack(name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if ds != nil {
					containers, _ := ds.GetStackContainers(r.Context(), name)
					if len(containers) > 0 {
						if err := ds.StopStack(r.Context(), name); err != nil {
							respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop stack: %v", err))
							return
						}
						respondJSON(w, http.StatusOK, map[string]string{"message": "stack stopped"})
						return
					}
				}
				respondJSONError(w, http.StatusNotFound, "stack not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if ds != nil {
			if err := ds.StopStack(r.Context(), name); err != nil {
				respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop stack: %v", err))
				return
			}
		}

		stack.Status = "stopped"
		stack.UpdatedAt = time.Now()
		if err := st.SaveStack(stack); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to update stack: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, stack)
	}
}

// HandleDownStack stops and removes containers and networks of a stack.
func HandleDownStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "stack name is required")
			return
		}

		stack, err := st.GetStack(name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if ds != nil {
					containers, _ := ds.GetStackContainers(r.Context(), name)
					if len(containers) > 0 {
						if err := ds.DownStack(r.Context(), name); err != nil {
							respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to bring down stack: %v", err))
							return
						}
						if cache := ds.GetStackDetailCache(); cache != nil {
							cache.Invalidate(name)
						}
						respondJSON(w, http.StatusOK, map[string]string{"message": "stack brought down"})
						return
					}
				}
				respondJSONError(w, http.StatusNotFound, "stack not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if ds != nil {
			if err := ds.DownStack(r.Context(), name); err != nil {
				respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to bring down stack: %v", err))
				return
			}
			if cache := ds.GetStackDetailCache(); cache != nil {
				cache.Invalidate(name)
			}
		}

		stack.Status = "down"
		stack.UpdatedAt = time.Now()
		if err := st.SaveStack(stack); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to update stack: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, stack)
	}
}

// HandleDeleteStack removes the stack definition and cleans up resources.
func HandleDeleteStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "stack name is required")
			return
		}

		_, err := st.GetStack(name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if ds != nil {
					containers, _ := ds.GetStackContainers(r.Context(), name)
					if len(containers) > 0 {
						_ = ds.DownStack(r.Context(), name)
						if cache := ds.GetStackDetailCache(); cache != nil {
							cache.Invalidate(name)
						}
						respondJSON(w, http.StatusOK, map[string]string{"message": "stack deleted"})
						return
					}
				}
				respondJSONError(w, http.StatusNotFound, "stack not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if ds != nil {
			_ = ds.DownStack(r.Context(), name)
			if cache := ds.GetStackDetailCache(); cache != nil {
				cache.Invalidate(name)
			}
		}

		if err := st.DeleteStack(name); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to delete stack: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "stack deleted"})
	}
}

// HandleAdoptStack elevates an external Docker Compose project into a fully-managed PDSwarm stack.
// It persists the stack definition into the store with IsExternal=false without restarting containers.
func HandleAdoptStack(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "stack name is required")
			return
		}

		var req AdoptStackRequest
		if r.Body != nil && r.ContentLength > 0 {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		yamlContent := strings.TrimSpace(req.ComposeYAML)
		envContent := strings.TrimSpace(req.Env)
		workingDir := strings.TrimSpace(req.WorkingDir)
		var configFile string
		var createdAt time.Time

		// Check if containers exist for this stack
		if ds != nil {
			containers, err := ds.GetStackContainers(r.Context(), name)
			if err == nil && len(containers) > 0 {
				diskYAML, diskEnv, cfgFile, wDir := extractComposeDetails(r.Context(), ds, name, containers)
				if yamlContent == "" {
					yamlContent = diskYAML
				}
				if workingDir == "" {
					workingDir = wDir
				}
				if envContent == "" {
					envContent = diskEnv
				}
				configFile = cfgFile
				if containers[0].Created > 0 {
					createdAt = time.Unix(containers[0].Created, 0)
				}
			}
		}

		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		// If still empty, check if it was previously saved in store
		if existing, err := st.GetStack(name); err == nil && existing != nil {
			if yamlContent == "" {
				yamlContent = existing.ComposeYAML
			}
			if envContent == "" {
				envContent = existing.Env
			}
			if workingDir == "" {
				workingDir = existing.WorkingDir
			}
			if configFile == "" {
				configFile = existing.ConfigFile
			}
		}

		if yamlContent == "" {
			respondJSONError(w, http.StatusBadRequest, "no compose YAML found on host or provided in request body")
			return
		}

		// Validate Compose YAML with env
		if _, err := docker.ParseComposeYAMLWithEnv(yamlContent, docker.ParseEnvFile(envContent)); err != nil {
			respondJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid compose YAML: %v", err))
			return
		}

		stack := &store.Stack{
			Name:        name,
			ComposeYAML: yamlContent,
			Env:         envContent,
			Status:      "running",
			IsExternal:  false, // Elevated to managed!
			WorkingDir:  workingDir,
			ConfigFile:  configFile,
			CreatedAt:   createdAt,
			UpdatedAt:   time.Now(),
		}

		if err := st.SaveStack(stack); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save adopted stack: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, stack)
	}
}
