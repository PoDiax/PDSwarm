package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
)

var (
	ErrTagRequired        = errors.New("image tag is required")
	ErrDockerfileRequired = errors.New("dockerfile content is required")
)

// BuildImageStream packages dockerfileContent into an in-memory tar archive and streams
// the Docker daemon's ImageBuild response output.
// The caller is responsible for reading and closing the returned io.ReadCloser.
func (s *Service) BuildImageStream(ctx context.Context, tag, dockerfileContent string) (io.ReadCloser, error) {
	return s.BuildImageStreamWithContext(ctx, tag, dockerfileContent, "")
}

// BuildImageStreamWithContext packages dockerfileContent and an optional host contextDir
// into a tar archive and streams the Docker daemon's ImageBuild response output.
func (s *Service) BuildImageStreamWithContext(ctx context.Context, tag, dockerfileContent, contextDir string) (io.ReadCloser, error) {
	if s.cli == nil {
		return nil, errors.New("docker client not available")
	}

	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, ErrTagRequired
	}

	if strings.TrimSpace(dockerfileContent) == "" {
		return nil, ErrDockerfileRequired
	}

	tarReader, err := createBuildContext(dockerfileContent, contextDir)
	if err != nil {
		return nil, err
	}

	resp, err := s.cli.ImageBuild(ctx, tarReader, types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
		Remove:     true,
		NoCache:    false,
	})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func createBuildContext(dockerfileContent, contextDir string) (io.Reader, error) {
	contextDir = strings.TrimSpace(contextDir)
	if contextDir == "" {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		content := []byte(dockerfileContent)
		hdr := &tar.Header{
			Name:    "Dockerfile",
			Mode:    0644,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
		if err := tw.Close(); err != nil {
			return nil, err
		}
		return bytes.NewReader(buf.Bytes()), nil
	}

	stat, err := os.Stat(contextDir)
	if err != nil {
		return nil, fmt.Errorf("context directory does not exist: %w", err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("context path is not a directory: %s", contextDir)
	}

	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		var writeErr error
		defer func() {
			if writeErr != nil {
				_ = tw.Close()
				_ = pw.CloseWithError(writeErr)
			} else {
				_ = tw.Close()
				_ = pw.Close()
			}
		}()

		dockerfileWritten := false

		writeErr = filepath.Walk(contextDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(contextDir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}

			tarPath := filepath.ToSlash(rel)

			if tarPath == "Dockerfile" {
				dockerfileWritten = true
				content := []byte(dockerfileContent)
				hdr := &tar.Header{
					Name:    "Dockerfile",
					Mode:    0644,
					Size:    int64(len(content)),
					ModTime: time.Now(),
				}
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				_, err = tw.Write(content)
				return err
			}

			var linkTarget string
			if info.Mode()&os.ModeSymlink != 0 {
				linkTarget, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}

			hdr, err := tar.FileInfoHeader(info, linkTarget)
			if err != nil {
				return err
			}
			hdr.Name = tarPath

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			if info.Mode().IsRegular() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(tw, f)
				_ = f.Close()
				if err != nil {
					return err
				}
			}
			return nil
		})

		if writeErr != nil {
			return
		}

		if !dockerfileWritten {
			content := []byte(dockerfileContent)
			hdr := &tar.Header{
				Name:    "Dockerfile",
				Mode:    0644,
				Size:    int64(len(content)),
				ModTime: time.Now(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				writeErr = err
				return
			}
			if _, err := tw.Write(content); err != nil {
				writeErr = err
				return
			}
		}
	}()

	return pr, nil
}
