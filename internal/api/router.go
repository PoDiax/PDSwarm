package api

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/podiax/pdswarm"
	"github.com/podiax/pdswarm/internal/auth"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/templates"
)

type contextKey string

const (
	contextKeyDockerService contextKey = "dockerService"
	contextKeyEnvironment   contextKey = "environment"
)

// EnvironmentMiddleware extracts X-Environment-ID header (or ?env=<id> query parameter)
// and injects the resolved *docker.Service and *store.Environment into request context.
func EnvironmentMiddleware(st store.Store, reg *docker.DockerServiceRegistry, defaultDS *docker.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			envID := r.Header.Get("X-Environment-ID")
			if envID == "" {
				envID = r.URL.Query().Get("env")
			}

			var activeDS *docker.Service
			var activeEnv *store.Environment

			if envID != "" {
				if reg != nil {
					if ds, err := reg.Get(envID); err == nil && ds != nil {
						activeDS = ds
					}
				}
				if activeDS == nil && st != nil {
					env, err := st.GetEnvironment(envID)
					if err == nil && env != nil {
						activeEnv = env
						if reg != nil {
							ds, err := reg.GetByEnv(r.Context(), env)
							if err == nil {
								activeDS = ds
							}
						}
					}
				}
			}

			if activeDS == nil {
				activeDS = defaultDS
			}
			if activeEnv == nil && st != nil {
				activeEnv, _ = st.GetDefaultEnvironment()
			}

			ctx := r.Context()
			if activeDS != nil {
				ctx = context.WithValue(ctx, contextKeyDockerService, activeDS)
			}
			if activeEnv != nil {
				ctx = context.WithValue(ctx, contextKeyEnvironment, activeEnv)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// dockerFromContext retrieves the active *docker.Service from context, falling back to defaultDS.
func dockerFromContext(ctx context.Context, defaultDS *docker.Service) *docker.Service {
	if ctx == nil {
		return defaultDS
	}
	if ds, ok := ctx.Value(contextKeyDockerService).(*docker.Service); ok && ds != nil {
		return ds
	}
	return defaultDS
}

// DockerFromContext is the exported version of dockerFromContext.
func DockerFromContext(ctx context.Context, defaultDS *docker.Service) *docker.Service {
	return dockerFromContext(ctx, defaultDS)
}

// EnvironmentFromContext retrieves the active *store.Environment from context if present.
func EnvironmentFromContext(ctx context.Context) *store.Environment {
	if ctx == nil {
		return nil
	}
	if env, ok := ctx.Value(contextKeyEnvironment).(*store.Environment); ok {
		return env
	}
	return nil
}

// SetupRouter configures the chi HTTP router with middlewares, REST endpoints, and WebSocket handlers.
func SetupRouter(cfg *config.Config, st store.Store, ds *docker.Service, staticFS ...fs.FS) http.Handler {
	reg := docker.NewDockerServiceRegistry()
	if ds != nil {
		reg.Set("local", ds)
		if st != nil {
			if defEnv, err := st.GetDefaultEnvironment(); err == nil && defEnv != nil {
				reg.Set(defEnv.ID, ds)
			}
		}
	}
	return SetupRouterWithRegistry(cfg, st, ds, reg, staticFS...)
}

// SetupRouterWithRegistry configures the chi HTTP router with an explicit DockerServiceRegistry.
func SetupRouterWithRegistry(cfg *config.Config, st store.Store, ds *docker.Service, reg *docker.DockerServiceRegistry, staticFS ...fs.FS) http.Handler {
	r := chi.NewRouter()

	// Base middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Environment-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	jwtSecret := ""
	noAuth := false
	if cfg != nil {
		jwtSecret = cfg.JWTSecret
		noAuth = cfg.NoAuth
	}
	if jwtSecret == "" {
		jwtSecret = "pdswarm-default-jwt-secret-key"
	}

	r.Route("/api", func(apiRouter chi.Router) {
		// Environment resolution middleware
		apiRouter.Use(EnvironmentMiddleware(st, reg, ds))

		// Public auth endpoints
		apiRouter.Get("/auth/status", HandleAuthStatus(cfg, st))
		apiRouter.Post("/auth/setup", HandleAuthSetup(cfg, st))
		apiRouter.Post("/auth/login", HandleAuthLogin(cfg, st))
		apiRouter.Post("/auth/logout", HandleAuthLogout())

		// Public health check and version
		apiRouter.Get("/system/ping", HandleSystemPing(ds))
		apiRouter.Get("/system/version", HandleSystemVersion())
		apiRouter.Get("/version", HandleSystemVersion())

		// Protected endpoints
		apiRouter.Group(func(protected chi.Router) {
			protected.Use(auth.AuthMiddleware(jwtSecret, noAuth, st))

			// Auth identity & user administration
			protected.Get("/auth/me", HandleAuthMe(cfg))
			protected.Put("/auth/password", HandleChangePassword(st))
			protected.Get("/users", HandleListUsers(st))
			protected.Post("/users", HandleCreateUser(st))
			protected.Delete("/users/{username}", HandleDeleteUser(st))

			// API Access Tokens
			protected.Get("/tokens", HandleListTokens(st))
			protected.Post("/tokens", HandleCreateToken(st))
			protected.Delete("/tokens/{id}", HandleDeleteToken(st))

			// Background Tasks
			protected.Get("/tasks", HandleListTasks(nil))
			protected.Get("/tasks/{id}", HandleGetTask(nil))
			protected.Post("/tasks/{id}/cancel", HandleCancelTask(nil))
			protected.Delete("/tasks/{id}", HandleDeleteTask(nil))

			// Environments
			protected.Get("/environments", HandleListEnvironments(st, reg))
			protected.Post("/environments", HandleCreateEnvironment(st, reg))
			protected.Post("/environments/test", HandleTestEnvironment())
			protected.Get("/environments/{id}", HandleGetEnvironment(st, reg))
			protected.Put("/environments/{id}", HandleUpdateEnvironment(st, reg))
			protected.Delete("/environments/{id}", HandleDeleteEnvironment(st, reg))
			protected.Post("/environments/{id}/ping", HandlePingEnvironment(st, reg))

			// System info, Port conflict monitoring, Host filesystem path suggestions & Daemon Logging
			protected.Get("/system/info", HandleSystemInfo(ds))
			protected.Get("/system/ports", HandleGetAllocatedPorts(ds))
			protected.Post("/system/ports/check", HandleCheckPortConflicts(ds))
			protected.Get("/system/host-paths", HandleSuggestHostPaths(ds))
			protected.Get("/system/logging", HandleGetLoggingConfig(ds, st))
			protected.Put("/system/logging", HandleUpdateLoggingConfig(ds, st))
			protected.Get("/system/update", HandleGetUpdateStatus(st))
			protected.Post("/system/update/check", HandleCheckUpdate(st))
			protected.Put("/system/update/auto", HandleSetAutoUpdate(st))
			protected.Post("/system/update/apply", HandleApplyUpdate(st))
			protected.Get("/system/audit-logs", HandleGetAuditLogs(cfg))

			// Containers
			protected.Get("/containers", HandleListContainers(ds))
			protected.Post("/containers", HandleCreateContainer(ds, st))
			protected.Post("/containers/prune", HandlePruneContainers(ds))
			protected.Get("/containers/{id}", HandleInspectContainer(ds))
			protected.Post("/containers/{id}/start", HandleStartContainer(ds))
			protected.Post("/containers/{id}/stop", HandleStopContainer(ds))
			protected.Post("/containers/{id}/restart", HandleRestartContainer(ds))
			protected.Post("/containers/{id}/pause", HandlePauseContainer(ds))
			protected.Post("/containers/{id}/unpause", HandleUnpauseContainer(ds))
			protected.Post("/containers/{id}/kill", HandleKillContainer(ds))
			protected.Post("/containers/{id}/update-image", HandleUpdateContainerImage(ds, st))
			protected.Post("/containers/{id}/recreate", HandleRecreateContainer(ds, st))
			protected.Delete("/containers/{id}", HandleDeleteContainer(ds))

			// Container Files
			protected.Get("/containers/{id}/files/stat", HandleStatContainerFile(ds))
			protected.Get("/containers/{id}/files/list", HandleListContainerFiles(ds))
			protected.Get("/containers/{id}/files/read", HandleReadContainerFile(ds))
			protected.Get("/containers/{id}/files/download", HandleDownloadContainerFile(ds))
			protected.Post("/containers/{id}/files/upload", HandleUploadContainerFile(ds))
			protected.Delete("/containers/{id}/files", HandleDeleteContainerFile(ds))

			// Registries
			protected.Get("/registries", HandleListRegistries(st))
			protected.Post("/registries", HandleCreateRegistry(st))
			protected.Get("/registries/{id}", HandleGetRegistry(st))
			protected.Put("/registries/{id}", HandleUpdateRegistry(st))
			protected.Delete("/registries/{id}", HandleDeleteRegistry(st))
			protected.Post("/registries/{id}/ping", HandlePingRegistry(st, ds))
			protected.Post("/registries/test", HandleTestRegistry(st, ds))

			// Images
			protected.Get("/images", HandleListImages(ds))
			protected.Post("/images/pull", HandlePullImage(st, ds))
			protected.Post("/images/check-update", HandleCheckImageUpdate(st, ds))
			protected.Post("/images/check-updates", HandleCheckImageUpdatesBatch(st, ds))
			protected.Get("/images/{id}/check-update", HandleCheckImageUpdate(st, ds))
			protected.Post("/images/build", HandleBuildImage(ds))
			protected.Post("/images/prune", HandlePruneImages(ds))
			protected.Delete("/images/{id}", HandleDeleteImage(ds))

			// Volumes
			protected.Get("/volumes", HandleListVolumes(ds))
			protected.Post("/volumes", HandleCreateVolume(ds))
			protected.Post("/volumes/prune", HandlePruneVolumes(ds))
			protected.Delete("/volumes/{name}", HandleDeleteVolume(ds))

			// Networks
			protected.Get("/networks", HandleListNetworks(ds))
			protected.Post("/networks", HandleCreateNetwork(ds))
			protected.Post("/networks/prune", HandlePruneNetworks(ds))
			protected.Delete("/networks/{id}", HandleDeleteNetwork(ds))

			// Stacks
			protected.Get("/stacks", HandleListStacks(st, ds))
			protected.Post("/stacks", HandleCreateStack(st, ds))
			protected.Get("/stacks/{name}", HandleGetStack(st, ds))
			protected.Put("/stacks/{name}", HandleUpdateStack(st, ds))
			protected.Post("/stacks/{name}/adopt", HandleAdoptStack(st, ds))
			protected.Post("/stacks/{name}/stop", HandleStopStack(st, ds))
			protected.Post("/stacks/{name}/down", HandleDownStack(st, ds))
			protected.Delete("/stacks/{name}", HandleDeleteStack(st, ds))

			// Templates (1-Click App Store)
			catalogManager := templates.NewCatalogManager()
			if st != nil {
				_ = catalogManager.Rebuild(context.Background(), nil, st)
			}
			protected.Get("/templates", HandleListTemplates(catalogManager))
			protected.Get("/templates/categories", HandleListTemplateCategories(catalogManager))
			protected.Get("/templates/settings", HandleGetTemplateSettings(st))
			protected.Put("/templates/settings", HandleUpdateTemplateSettings(catalogManager, st))
			protected.Get("/templates/repos", HandleListTemplateRepos(st))
			protected.Post("/templates/repos", HandleCreateTemplateRepo(catalogManager, st))
			protected.Put("/templates/repos/{id}", HandleUpdateTemplateRepo(catalogManager, st))
			protected.Delete("/templates/repos/{id}", HandleDeleteTemplateRepo(catalogManager, st))
			protected.Post("/templates/repos/{id}/sync", HandleSyncTemplateRepo(catalogManager, st))
			protected.Post("/templates/repos/sync", HandleSyncAllTemplateRepos(catalogManager, st))
			protected.Get("/templates/{id}", HandleGetTemplate(catalogManager))
			protected.Post("/templates/deploy", HandleDeployTemplate(catalogManager, st, ds))

			// WebSockets
			protected.Get("/ws/containers/{id}/exec", HandleContainerExecWS(ds))
			protected.Get("/ws/containers/{id}/logs", HandleContainerLogsWS(ds))
			protected.Get("/ws/containers/{id}/stats", HandleContainerStatsWS(ds))
		})

		apiRouter.NotFound(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		})
	})

	var sfs fs.FS
	if len(staticFS) > 0 {
		sfs = staticFS[0]
	} else {
		sfs = pdswarm.WebFS()
	}

	if sfs != nil {
		r.Handle("/*", ServeStatic(sfs))
	}

	return r
}
