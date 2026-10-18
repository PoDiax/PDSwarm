package docker

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
)

var (
	ErrImageRefRequired = errors.New("image reference is required")
	ErrImageIDRequired  = errors.New("image ID is required")
)

// EncodeRegistryAuth encodes credentials for a Docker registry into a base64-encoded JSON string
// suitable for the X-Registry-Auth header or image.PullOptions.RegistryAuth.
func EncodeRegistryAuth(username, password, serverAddress string) string {
	if username == "" && password == "" {
		return ""
	}
	authConfig := registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}
	encoded, err := registry.EncodeAuthConfig(authConfig)
	if err != nil {
		return ""
	}
	return encoded
}

// ListImages returns a list of images configured in the docker host.
func (s *Service) ListImages(ctx context.Context) ([]image.Summary, error) {
	return s.cli.ImageList(ctx, image.ListOptions{})
}

// PullImageStream requests the docker host to pull an image from a remote registry with optional auth credentials.
// The caller is responsible for reading and closing the returned io.ReadCloser.
func (s *Service) PullImageStream(ctx context.Context, ref string, authConfig string) (io.ReadCloser, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, ErrImageRefRequired
	}
	opts := image.PullOptions{}
	if authConfig != "" {
		opts.RegistryAuth = authConfig
	}
	return s.cli.ImagePull(ctx, ref, opts)
}

// PullImage requests the docker host to pull an image from a remote registry without authentication.
// The caller is responsible for reading and closing the returned io.ReadCloser.
func (s *Service) PullImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	return s.PullImageStream(ctx, ref, "")
}

// RegistryLogin authenticates with a Docker registry using the docker daemon.
func (s *Service) RegistryLogin(ctx context.Context, username, password, serverAddress string) (string, error) {
	if s.cli == nil {
		return "", errors.New("docker client not available")
	}
	auth := registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}
	resp, err := s.cli.RegistryLogin(ctx, auth)
	if err != nil {
		return "", err
	}
	return resp.Status, nil
}

// RemoveImage removes an image from the docker host.
func (s *Service) RemoveImage(ctx context.Context, id string, force bool) error {
	if strings.TrimSpace(id) == "" {
		return ErrImageIDRequired
	}
	_, err := s.cli.ImageRemove(ctx, id, image.RemoveOptions{
		Force: force,
	})
	return err
}

// PruneImages deletes unused images from the docker host.
func (s *Service) PruneImages(ctx context.Context) (image.PruneReport, error) {
	return s.cli.ImagesPrune(ctx, filters.NewArgs())
}

// InspectImage inspects a Docker image and returns its metadata including Config.Env.
func (s *Service) InspectImage(ctx context.Context, id string) (types.ImageInspect, error) {
	if s.cli == nil {
		return types.ImageInspect{}, errors.New("docker client not available")
	}
	if strings.TrimSpace(id) == "" {
		return types.ImageInspect{}, ErrImageIDRequired
	}
	inspect, _, err := s.cli.ImageInspectWithRaw(ctx, id)
	return inspect, err
}

// GetImageEnv returns the baked environment variables of an image.
func (s *Service) GetImageEnv(ctx context.Context, id string) []string {
	if s.cli == nil || strings.TrimSpace(id) == "" {
		return []string{}
	}
	inspect, _, err := s.cli.ImageInspectWithRaw(ctx, id)
	if err != nil || inspect.Config == nil || len(inspect.Config.Env) == 0 {
		return []string{}
	}
	return inspect.Config.Env
}

// ImageUpdateInfo represents the status of a local image compared to its remote registry manifest.
type ImageUpdateInfo struct {
	ImageRef     string `json:"image_ref"`
	Status       string `json:"status"` // "up_to_date", "update_available", "not_found", "unknown"
	UpToDate     bool   `json:"up_to_date"`
	LocalDigest  string `json:"local_digest,omitempty"`
	RemoteDigest string `json:"remote_digest,omitempty"`
	Message      string `json:"message"`
	Error        string `json:"error,omitempty"`
}

// CheckImageUpdate compares the local image's digest with the registry's distribution manifest.
func (s *Service) CheckImageUpdate(ctx context.Context, imageRef, authConfig string) (*ImageUpdateInfo, error) {
	if s.cli == nil {
		return nil, errors.New("docker client not available")
	}
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return nil, ErrImageRefRequired
	}

	res := &ImageUpdateInfo{
		ImageRef: imageRef,
		Status:   "unknown",
	}

	// 1. Inspect local image
	inspect, _, err := s.cli.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		res.Status = "not_found"
		res.Message = "Local image not found: " + err.Error()
		return res, nil
	}

	var localDigest string
	if len(inspect.RepoDigests) > 0 {
		// e.g. "nginx@sha256:4b4976472f132e0325f4625b172a6b47c0b02018898ee3e8784a0a4cb207ebf0"
		localDigest = inspect.RepoDigests[0]
		if idx := strings.Index(localDigest, "@"); idx != -1 {
			localDigest = localDigest[idx+1:]
		}
	}
	res.LocalDigest = localDigest

	// 2. Query registry distribution manifest without pulling
	dist, err := s.cli.DistributionInspect(ctx, imageRef, authConfig)
	if err != nil {
		res.Status = "unknown"
		res.Error = err.Error()
		res.Message = "Could not reach registry: " + err.Error()
		return res, nil
	}

	remoteDigest := string(dist.Descriptor.Digest)
	res.RemoteDigest = remoteDigest

	// 3. Compare remote digest with all RepoDigests of the local image
	match := false
	if remoteDigest != "" {
		for _, rd := range inspect.RepoDigests {
			if strings.HasSuffix(rd, "@"+remoteDigest) || rd == remoteDigest {
				match = true
				break
			}
		}
		if !match && localDigest != "" && localDigest == remoteDigest {
			match = true
		}
	}

	if match {
		res.Status = "up_to_date"
		res.UpToDate = true
		res.Message = "Image is up to date"
	} else if remoteDigest != "" {
		res.Status = "update_available"
		res.UpToDate = false
		res.Message = "A newer image version is available on the registry"
	} else {
		res.Status = "unknown"
		res.Message = "Unable to determine remote image digest"
	}

	return res, nil
}


