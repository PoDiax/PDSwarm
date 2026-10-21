package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
)

type Service struct {
	cli        *client.Client
	stackCache *StackDetailCache
	cacheOnce  sync.Once
}

func NewService(socketPath string) (*Service, error) {
	var opts []client.Opt
	opts = append(opts, client.FromEnv)

	if socketPath != "" {
		host := socketPath
		if strings.HasPrefix(host, "/") {
			host = "unix://" + host
		}
		opts = append(opts, client.WithHost(host))
	} else if os.Getenv("DOCKER_HOST") == "" {
		opts = append(opts, client.WithHost("unix:///var/run/docker.sock"))
	}

	opts = append(opts, client.WithAPIVersionNegotiation())

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Service{
		cli:        cli,
		stackCache: NewStackDetailCache(10 * time.Minute),
	}, nil
}

func NewServiceWithClient(cli *client.Client) *Service {
	return &Service{
		cli:        cli,
		stackCache: NewStackDetailCache(10 * time.Minute),
	}
}

func (s *Service) GetStackDetailCache() *StackDetailCache {
	if s == nil {
		return nil
	}
	s.cacheOnce.Do(func() {
		if s.stackCache == nil {
			s.stackCache = NewStackDetailCache(10 * time.Minute)
		}
	})
	return s.stackCache
}

func (s *Service) Ping(ctx context.Context) error {
	_, err := s.cli.Ping(ctx)
	return err
}

func (s *Service) RawClient() *client.Client {
	return s.cli
}

func (s *Service) Close() error {
	if s.cli != nil {
		return s.cli.Close()
	}
	return nil
}
