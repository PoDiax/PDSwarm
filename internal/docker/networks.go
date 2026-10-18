package docker

import (
	"context"
	"errors"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

var (
	ErrNetworkNameRequired = errors.New("network name is required")
	ErrNetworkIDRequired   = errors.New("network ID is required")
)

// ListNetworks returns the list of networks configured in the docker host.
func (s *Service) ListNetworks(ctx context.Context) ([]network.Summary, error) {
	return s.cli.NetworkList(ctx, network.ListOptions{})
}

// CreateNetwork creates a new network in the docker host and returns its ID.
func (s *Service) CreateNetwork(ctx context.Context, name, driver string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", ErrNetworkNameRequired
	}
	resp, err := s.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: driver,
	})
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// RemoveNetwork removes an existent network from the docker host.
func (s *Service) RemoveNetwork(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrNetworkIDRequired
	}
	return s.cli.NetworkRemove(ctx, id)
}

// PruneNetworks deletes unused networks from the docker host.
func (s *Service) PruneNetworks(ctx context.Context) (network.PruneReport, error) {
	return s.cli.NetworksPrune(ctx, filters.NewArgs())
}
