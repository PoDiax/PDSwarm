package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/podiax/pdswarm/internal/store"
)

// DockerServiceRegistry maintains a thread-safe cache of active Docker service clients
// indexed by environment ID.
type DockerServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*Service
}

// Registry is an alias for DockerServiceRegistry.
type Registry = DockerServiceRegistry

// NewDockerServiceRegistry instantiates an empty Docker service registry.
func NewDockerServiceRegistry() *DockerServiceRegistry {
	return &DockerServiceRegistry{
		services: make(map[string]*Service),
	}
}

// NewRegistry is an alias for NewDockerServiceRegistry.
func NewRegistry() *DockerServiceRegistry {
	return NewDockerServiceRegistry()
}

// NewServiceFromEnvironment constructs a new *Service instance for a given environment configuration.
func NewServiceFromEnvironment(env *store.Environment) (*Service, error) {
	if env == nil {
		return nil, fmt.Errorf("environment cannot be nil")
	}

	var opts []client.Opt
	opts = append(opts, client.WithAPIVersionNegotiation())

	switch env.Type {
	case "local":
		host := env.URL
		if host == "" {
			host = os.Getenv("DOCKER_HOST")
			if host == "" {
				host = "unix:///var/run/docker.sock"
			}
		}
		if strings.HasPrefix(host, "/") {
			host = "unix://" + host
		}
		opts = append(opts, client.WithHost(host))

	case "tcp":
		host := env.URL
		if host == "" {
			return nil, fmt.Errorf("url is required for tcp environment")
		}
		if !strings.Contains(host, "://") {
			host = "tcp://" + host
		}
		opts = append(opts, client.WithHost(host))

	case "tls":
		host := env.URL
		if host == "" {
			return nil, fmt.Errorf("url is required for tls environment")
		}
		if !strings.Contains(host, "://") {
			host = "tcp://" + host
		}
		opts = append(opts, client.WithHost(host))

		caPath, caClean, err := resolveTLSFile(env.TLSCA, "ca")
		if err != nil {
			return nil, fmt.Errorf("failed to process TLS CA: %w", err)
		}
		defer caClean()

		certPath, certClean, err := resolveTLSFile(env.TLSCert, "cert")
		if err != nil {
			return nil, fmt.Errorf("failed to process TLS cert: %w", err)
		}
		defer certClean()

		keyPath, keyClean, err := resolveTLSFile(env.TLSKey, "key")
		if err != nil {
			return nil, fmt.Errorf("failed to process TLS key: %w", err)
		}
		defer keyClean()

		if caPath != "" || certPath != "" || keyPath != "" {
			opts = append(opts, client.WithTLSClientConfig(caPath, certPath, keyPath))
		}

	default:
		return nil, fmt.Errorf("unsupported environment type: %q", env.Type)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize docker client: %w", err)
	}

	return &Service{cli: cli}, nil
}

// resolveTLSFile checks if the provided string is an existing file path or raw certificate data.
// If raw data, it writes it to a temporary file with restricted permissions and returns a cleanup func.
func resolveTLSFile(dataOrPath, prefix string) (string, func(), error) {
	if strings.TrimSpace(dataOrPath) == "" {
		return "", func() {}, nil
	}

	if _, err := os.Stat(dataOrPath); err == nil {
		return dataOrPath, func() {}, nil
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("pdswarm-tls-%s-*.pem", prefix))
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temp tls file: %w", err)
	}

	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", func() {}, fmt.Errorf("failed to chmod temp tls file: %w", err)
	}

	if _, err := tmpFile.WriteString(dataOrPath); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", func() {}, fmt.Errorf("failed to write temp tls file: %w", err)
	}
	_ = tmpFile.Close()

	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}
	return tmpFile.Name(), cleanup, nil
}

// Get returns the cached *Service for the given environment ID, matching the docker.Registry interface.
func (r *DockerServiceRegistry) Get(id string) (*Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[id]
	if !ok || svc == nil {
		return nil, fmt.Errorf("service not found for environment: %s", id)
	}
	return svc, nil
}

// GetByID is an alias for Get(id).
func (r *DockerServiceRegistry) GetByID(id string) (*Service, error) {
	return r.Get(id)
}

// GetByEnv returns the cached *Service for the given environment without pinging,
// or initializes, pings, and caches a new one if not present. Network I/O is performed
// outside of the mutex lock to avoid lock contention.
func (r *DockerServiceRegistry) GetByEnv(ctx context.Context, env *store.Environment) (*Service, error) {
	if env == nil {
		return nil, fmt.Errorf("environment cannot be nil")
	}

	// 1. Fast path: check cache under RLock. Return immediately if cached (no ping on each request).
	r.mu.RLock()
	svc, exists := r.services[env.ID]
	r.mu.RUnlock()

	if exists && svc != nil {
		return svc, nil
	}

	// 2. Cache miss: initialize client outside of mutex lock
	newSvc, err := NewServiceFromEnvironment(env)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client for %s: %w", env.ID, err)
	}

	// Probe ping with timeout OUTSIDE of lock so we don't block other goroutines
	if ctx != nil {
		pingCtx := ctx
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			pingCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
		}
		if err := newSvc.Ping(pingCtx); err != nil {
			_ = newSvc.Close()
			return nil, fmt.Errorf("ping validation failed for %s: %w", env.ID, err)
		}
	}

	// 3. Briefly acquire write lock only to insert into map
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check if another goroutine cached it while we were pinging
	if existing, exists := r.services[env.ID]; exists && existing != nil {
		_ = newSvc.Close()
		return existing, nil
	}

	r.services[env.ID] = newSvc
	return newSvc, nil
}

// Register connects to the environment with ping validation and caches the client connection.
func (r *DockerServiceRegistry) Register(env *store.Environment) (*Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.GetByEnv(ctx, env)
}

// Set explicitly sets or replaces a service instance in the cache (useful for mocking/testing).
func (r *DockerServiceRegistry) Set(id string, svc *Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[id] = svc
}

// Remove closes and deletes the client associated with the given environment ID.
func (r *DockerServiceRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if svc, ok := r.services[id]; ok && svc != nil {
		_ = svc.Close()
	}
	delete(r.services, id)
}

// Close closes all cached Docker clients and empties the registry.
func (r *DockerServiceRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []error
	for id, svc := range r.services {
		if svc != nil {
			if err := svc.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		delete(r.services, id)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing registry services: %v", errs)
	}
	return nil
}
