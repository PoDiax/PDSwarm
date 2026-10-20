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
	"golang.org/x/time/rate"
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

	var trustedProxies []string
	var allowedOrigins []string
	if cfg != nil {
		trustedProxies = cfg.TrustedProxies
		allowedOrigins = cfg.AllowedOrigins
	}

	// Base middlewares
	r.Use(middleware.RequestID)
	r.Use(TrustedProxyMiddleware(trustedProxies))
	r.Use(RateLimitMiddleware(NewIPRateLimiter(rate.Limit(100), 200))) // Global rate limit: 100 req/s, burst 200
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Strict CORS middleware - allows specified origins or same-origin only when empty
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins, // Empty means same-origin only for browser security
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

	authLimiter := NewIPRateLimiter(rate.Limit(5), 10) // Auth-specific rate limit: 5 req/s, burst 10

	r.Route("/api", func(apiRouter chi.Router) {
		// Environment resolution middleware
		apiRouter.Use(EnvironmentMiddleware(st, reg, ds))

		// Public auth endpoints
		apiRouter.Group(func(authRouter chi.Router) {
			authRouter.Use(RateLimitMiddleware(authLimiter))
			authRouter.Get("/auth/status", HandleAuthStatus(cfg, st))
			authRouter.Post("/auth/setup", HandleAuthSetup(cfg, st))
			authRouter.Post("/auth/login", HandleAuthLogin(cfg, st))
			authRouter.Post("/auth/logout", HandleAuthLogout())
		})

		// Public health check and version
		apiRouter.Get("/system/ping", HandleSystemPing(ds))
		apiRouter.Get("/system/version", HandleSystemVersion())
		apiRouter.Get("/version", HandleSystemVersion())

		// Initialize catalog manager for templates
		catalogManager := templates.NewCatalogManager()
		if st != nil {
			_ = catalogManager.Rebuild(context.Background(), nil, st)
		}

		// Protected endpoints
		apiRouter.Group(func(protected chi.Router) {
			protected.Use(auth.AuthMiddleware(jwtSecret, noAuth, st))

			// --- ADMIN ONLY ROUTES ---
			protected.Group(func(admin chi.Router) {
				admin.Use(RequireRole("admin"))

				// Users
				admin.Get("/users", HandleListUsers(st))
				admin.Post("/users", HandleCreateUser(st))
				admin.Delete("/users/{username}", HandleDeleteUser(st))

				// Environments
				admin.Post("/environments", HandleCreateEnvironment(st, reg))
				admin.Post("/environments/test", HandleTestEnvironment())
				admin.Put("/environments/{id}", HandleUpdateEnvironment(st, reg))
				admin.Delete("/environments/{id}", HandleDeleteEnvironment(st, reg))

				// System info, Host paths & Daemon Logging
				admin.Get("/system/host-paths", HandleSuggestHostPaths(ds))
				admin.Get("/system/logging", HandleGetLoggingConfig(ds, st))
				admin.Put("/system/logging", HandleUpdateLoggingConfig(ds, st))
				admin.Get("/system/update", HandleGetUpdateStatus(st, ds))
				admin.Post("/system/update/check", HandleCheckUpdate(st))
				admin.Put("/system/update/auto", HandleSetAutoUpdate(st, ds))
				admin.Post("/system/update/apply", HandleApplyUpdate(st, ds))
				admin.Get("/system/audit-logs", HandleGetAuditLogs(cfg))

				// Registries
				admin.Post("/registries", HandleCreateRegistry(st))
				admin.Put("/registries/{id}", HandleUpdateRegistry(st))
				admin.Delete("/registries/{id}", HandleDeleteRegistry(st))
				admin.Post("/registries/{id}/ping", HandlePingRegistry(st, ds))

				// Template Repositories & Settings
				admin.Get("/templates/settings", HandleGetTemplateSettings(st))
				admin.Put("/templates/settings", HandleUpdateTemplateSettings(catalogManager, st))
				admin.Get("/templates/repos", HandleListTemplateRepos(st))
				admin.Post("/templates/repos", HandleCreateTemplateRepo(catalogManager, st))
				admin.Put("/templates/repos/{id}", HandleUpdateTemplateRepo(catalogManager, st))
				admin.Delete("/templates/repos/{id}", HandleDeleteTemplateRepo(catalogManager, st))
				admin.Post("/templates/repos/{id}/sync", HandleSyncTemplateRepo(catalogManager, st))
				admin.Post("/templates/repos/sync", HandleSyncAllTemplateRepos(catalogManager, st))
			})

			// --- OPERATOR & ADMIN ROUTES ---
			protected.Group(func(op chi.Router) {
				op.Use(RequireRole("admin", "operator"))

				// Containers
				op.Post("/containers", HandleCreateContainer(ds, st))
				op.Post("/containers/prune", HandlePruneContainers(ds))
				op.Post("/containers/{id}/start", HandleStartContainer(ds))
				op.Post("/containers/{id}/stop", HandleStopContainer(ds))
				op.Post("/containers/{id}/restart", HandleRestartContainer(ds))
				op.Post("/containers/{id}/pause", HandlePauseContainer(ds))
				op.Post("/containers/{id}/unpause", HandleUnpauseContainer(ds))
				op.Post("/containers/{id}/kill", HandleKillContainer(ds))
				op.Post("/containers/{id}/update-image", HandleUpdateContainerImage(ds, st))
				op.Post("/containers/{id}/recreate", HandleRecreateContainer(ds, st))
				op.Delete("/containers/{id}", HandleDeleteContainer(ds))

				// Container Files (Write)
				op.Post("/containers/{id}/files/upload", HandleUploadContainerFile(ds))
				op.Delete("/containers/{id}/files", HandleDeleteContainerFile(ds))

				// Images (Mutations)
				op.Post("/images/pull", HandlePullImage(st, ds))
				op.Post("/images/build", HandleBuildImage(ds))
				op.Post("/images/prune", HandlePruneImages(ds))
				op.Delete("/images/{id}", HandleDeleteImage(ds))

				// Volumes (Mutations)
				op.Post("/volumes", HandleCreateVolume(ds))
				op.Post("/volumes/prune", HandlePruneVolumes(ds))
				op.Delete("/volumes/{name}", HandleDeleteVolume(ds))

				// Networks (Mutations)
				op.Post("/networks", HandleCreateNetwork(ds))
				op.Post("/networks/prune", HandlePruneNetworks(ds))
				op.Delete("/networks/{id}", HandleDeleteNetwork(ds))

				// Stacks (Mutations)
				op.Post("/stacks", HandleCreateStack(st, ds))
				op.Put("/stacks/{name}", HandleUpdateStack(st, ds))
				op.Post("/stacks/{name}/adopt", HandleAdoptStack(st, ds))
				op.Post("/stacks/{name}/stop", HandleStopStack(st, ds))
				op.Post("/stacks/{name}/down", HandleDownStack(st, ds))
				op.Delete("/stacks/{name}", HandleDeleteStack(st, ds))

				// Templates (Deploy)
				op.Post("/templates/deploy", HandleDeployTemplate(catalogManager, st, ds))

				// Terminal Exec (Write/Shell access)
				op.Get("/ws/containers/{id}/exec", HandleContainerExecWS(ds))
			})

			// --- ALL AUTHENTICATED USERS (Viewer, Operator, Admin) ---
			
			// Identity & Access Tokens
			protected.Get("/auth/me", HandleAuthMe(cfg))
			protected.Put("/auth/password", HandleChangePassword(st))
			protected.Get("/tokens", HandleListTokens(st))
			protected.Post("/tokens", HandleCreateToken(st))
			protected.Delete("/tokens/{id}", HandleDeleteToken(st))

			// Background Tasks
			protected.Get("/tasks", HandleListTasks(nil))
			protected.Get("/tasks/{id}", HandleGetTask(nil))
			protected.Post("/tasks/{id}/cancel", HandleCancelTask(nil))
			protected.Delete("/tasks/{id}", HandleDeleteTask(nil))

			// Environments (Read-only for Viewers)
			protected.Get("/environments", HandleListEnvironments(st, reg))
			protected.Get("/environments/{id}", HandleGetEnvironment(st, reg))
			protected.Post("/environments/{id}/ping", HandlePingEnvironment(st, reg))

			// System Ports
			protected.Get("/system/info", HandleSystemInfo(ds))
			protected.Get("/system/ports", HandleGetAllocatedPorts(ds))
			protected.Post("/system/ports/check", HandleCheckPortConflicts(ds))

			// Containers (Read-only)
			protected.Get("/containers", HandleListContainers(ds))
			protected.Get("/containers/{id}", HandleInspectContainer(ds))

			// Container Files (Read-only)
			protected.Get("/containers/{id}/files/stat", HandleStatContainerFile(ds))
			protected.Get("/containers/{id}/files/list", HandleListContainerFiles(ds))
			protected.Get("/containers/{id}/files/read", HandleReadContainerFile(ds))
			protected.Get("/containers/{id}/files/download", HandleDownloadContainerFile(ds))

			// Registries (Read-only)
			protected.Get("/registries", HandleListRegistries(st))
			protected.Get("/registries/{id}", HandleGetRegistry(st))
			protected.Post("/registries/test", HandleTestRegistry(st, ds))

			// Images (Read-only)
			protected.Get("/images", HandleListImages(ds))
			protected.Post("/images/check-update", HandleCheckImageUpdate(st, ds))
			protected.Post("/images/check-updates", HandleCheckImageUpdatesBatch(st, ds))
			protected.Get("/images/{id}/check-update", HandleCheckImageUpdate(st, ds))

			// Volumes (Read-only)
			protected.Get("/volumes", HandleListVolumes(ds))

			// Networks (Read-only)
			protected.Get("/networks", HandleListNetworks(ds))

			// Stacks (Read-only)
			protected.Get("/stacks", HandleListStacks(st, ds))
			protected.Get("/stacks/{name}", HandleGetStack(st, ds))

			// Templates (Read-only)
			protected.Get("/templates", HandleListTemplates(catalogManager))
			protected.Get("/templates/categories", HandleListTemplateCategories(catalogManager))
			protected.Get("/templates/{id}", HandleGetTemplate(catalogManager))

			// WebSockets (Read-only)
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
