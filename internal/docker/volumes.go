package docker

import (
	"context"
	"errors"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

var (
	ErrVolumeNameRequired = errors.New("volume name is required")
)

// ListVolumes returns the volumes configured in the docker host with calculated size & usage data.
func (s *Service) ListVolumes(ctx context.Context) (volume.ListResponse, error) {
	resp, err := s.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return volume.ListResponse{}, err
	}

	// Enrich volumes with UsageData (size & refcount) from Docker's DiskUsage API
	du, duErr := s.cli.DiskUsage(ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{types.VolumeObject},
	})
	if duErr != nil {
		du, duErr = s.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	}
	if duErr == nil && len(du.Volumes) > 0 {
		usageMap := make(map[string]*volume.UsageData, len(du.Volumes))
		for _, v := range du.Volumes {
			if v != nil && v.UsageData != nil {
				usageMap[v.Name] = v.UsageData
			}
		}
		for _, v := range resp.Volumes {
			if v != nil {
				if ud, ok := usageMap[v.Name]; ok {
					v.UsageData = ud
				}
			}
		}
	}

	return resp, nil
}

// CreateVolume creates a volume in the docker host.
func (s *Service) CreateVolume(ctx context.Context, name string, labels map[string]string) (volume.Volume, error) {
	if strings.TrimSpace(name) == "" {
		return volume.Volume{}, ErrVolumeNameRequired
	}
	return s.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name:   name,
		Labels: labels,
	})
}

// RemoveVolume removes a volume from the docker host.
func (s *Service) RemoveVolume(ctx context.Context, name string, force bool) error {
	if strings.TrimSpace(name) == "" {
		return ErrVolumeNameRequired
	}
	return s.cli.VolumeRemove(ctx, name, force)
}

// PruneVolumes deletes unused volumes from the docker host.
func (s *Service) PruneVolumes(ctx context.Context) (volume.PruneReport, error) {
	return s.cli.VolumesPrune(ctx, filters.NewArgs())
}
