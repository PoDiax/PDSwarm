package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// FileInfo represents metadata for a file or directory inside a container.
type FileInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModTime    time.Time `json:"mod_time"`
	IsDir      bool      `json:"is_dir"`
	LinkTarget string    `json:"link_target,omitempty"`
}

type tarFileReader struct {
	r      io.Reader
	closer io.Closer
}

func (t *tarFileReader) Read(p []byte) (int, error) {
	return t.r.Read(p)
}

func (t *tarFileReader) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}
	return nil
}

// StatContainerPath returns path stat information for a container filesystem path.
func (s *Service) StatContainerPath(ctx context.Context, id, targetPath string) (types.ContainerPathStat, error) {
	if s == nil || s.cli == nil {
		return types.ContainerPathStat{}, errors.New("docker service not available")
	}
	cleanPath := path.Clean(targetPath)
	if cleanPath == "" || cleanPath == "." {
		cleanPath = "/"
	}
	return s.cli.ContainerStatPath(ctx, id, cleanPath)
}

// ListContainerFiles lists entries inside dirPath (non-recursive, 1 level).
// If container is running, it attempts fast non-recursive inspection via exec.
// If exec is unavailable, container is stopped, or minimal/scratch image,
// it falls back to CopyFromContainer with tar stream header scanning.
func (s *Service) ListContainerFiles(ctx context.Context, id, dirPath string) ([]FileInfo, error) {
	if s == nil || s.cli == nil {
		return nil, errors.New("docker service not available")
	}

	cleanPath := path.Clean(dirPath)
	if cleanPath == "" || cleanPath == "." {
		cleanPath = "/"
	}
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// Try exec if container is running
	inspect, err := s.cli.ContainerInspect(ctx, id)
	if err == nil && inspect.State != nil && inspect.State.Running {
		files, execErr := s.listContainerFilesExec(ctx, id, cleanPath)
		if execErr == nil {
			sortFiles(files)
			return files, nil
		}
	}

	// Fallback to CopyFromContainer with tar stream parsing
	files, err := s.listContainerFilesTar(ctx, id, cleanPath)
	if err != nil {
		return nil, err
	}
	sortFiles(files)
	return files, nil
}

func (s *Service) listContainerFilesExec(ctx context.Context, id, cleanPath string) ([]FileInfo, error) {
	// Shell script to list entries within directory with tab-separated attributes
	// Field format: <size>\t<mtime_sec>\t<mode_str>\t<link_target>\t<name>
	shScript := `cd "$1" 2>/dev/null || exit 1
for f in .* *; do
    [ "$f" = "." ] || [ "$f" = ".." ] && continue
    [ -e "$f" ] || [ -L "$f" ] || continue
    target=""
    [ -L "$f" ] && target=$(readlink "$f" 2>/dev/null || true)
    stat -c "%s	%Y	%A	$target	%n" "$f" 2>/dev/null || true
done`

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", shScript, "_", cleanPath},
	}

	execResp, err := s.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := s.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach exec: %w", err)
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)

	inspectResp, err := s.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}
	if inspectResp.ExitCode != 0 {
		return nil, fmt.Errorf("exec listing failed with exit code %d: %s", inspectResp.ExitCode, strings.TrimSpace(stderr.String()))
	}

	var files []FileInfo
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}
		size, _ := strconv.ParseInt(parts[0], 10, 64)
		sec, _ := strconv.ParseInt(parts[1], 10, 64)
		mode := parts[2]
		linkTarget := parts[3]
		name := parts[4]

		isDir := strings.HasPrefix(mode, "d")
		modTime := time.Unix(sec, 0)

		files = append(files, FileInfo{
			Name:       name,
			Path:       path.Join(cleanPath, name),
			Size:       size,
			Mode:       mode,
			ModTime:    modTime,
			IsDir:      isDir,
			LinkTarget: linkTarget,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (s *Service) listContainerFilesTar(ctx context.Context, id, cleanPath string) ([]FileInfo, error) {
	reader, _, err := s.cli.CopyFromContainer(ctx, id, cleanPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	var prefix string
	first := true
	entriesMap := make(map[string]*FileInfo)
	var order []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := strings.TrimPrefix(hdr.Name, "/")
		name = strings.TrimSuffix(name, "/")
		if first {
			first = false
			// First header is typically the directory itself or "."
			if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/") || name == path.Base(cleanPath) || name == "." {
				prefix = name
				continue
			}
		}

		rel := name
		if prefix != "" && prefix != "." {
			if rel == prefix {
				continue
			}
			if strings.HasPrefix(rel, prefix+"/") {
				rel = strings.TrimPrefix(rel, prefix+"/")
			}
		}

		rel = strings.Trim(rel, "/")
		if rel == "" || rel == "." || rel == ".." {
			continue
		}

		slashIdx := strings.Index(rel, "/")
		if slashIdx != -1 {
			topName := rel[:slashIdx]
			if _, exists := entriesMap[topName]; !exists {
				order = append(order, topName)
				entriesMap[topName] = &FileInfo{
					Name:    topName,
					Path:    path.Join(cleanPath, topName),
					IsDir:   true,
					Mode:    "drwxr-xr-x",
					ModTime: hdr.ModTime,
				}
			}
			continue
		}

		fi := &FileInfo{
			Name:       rel,
			Path:       path.Join(cleanPath, rel),
			Size:       hdr.Size,
			Mode:       hdr.FileInfo().Mode().String(),
			ModTime:    hdr.ModTime,
			IsDir:      hdr.FileInfo().IsDir() || hdr.Typeflag == tar.TypeDir,
			LinkTarget: hdr.Linkname,
		}
		if _, exists := entriesMap[rel]; !exists {
			order = append(order, rel)
		}
		entriesMap[rel] = fi
	}

	var files []FileInfo
	for _, name := range order {
		if fi, ok := entriesMap[name]; ok {
			files = append(files, *fi)
		}
	}
	return files, nil
}

func sortFiles(files []FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		lowI := strings.ToLower(files[i].Name)
		lowJ := strings.ToLower(files[j].Name)
		if lowI != lowJ {
			return lowI < lowJ
		}
		return files[i].Name < files[j].Name
	})
}

// ReadContainerFile reads up to maxBytes from filePath inside a container using CopyFromContainer.
func (s *Service) ReadContainerFile(ctx context.Context, id, filePath string, maxBytes int64) ([]byte, *FileInfo, error) {
	if s == nil || s.cli == nil {
		return nil, nil, errors.New("docker service not available")
	}

	cleanPath := path.Clean(filePath)
	reader, _, err := s.cli.CopyFromContainer(ctx, id, cleanPath)
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if hdr.FileInfo().IsDir() {
			return nil, nil, fmt.Errorf("cannot read %s: path is a directory", cleanPath)
		}

		fi := &FileInfo{
			Name:       path.Base(cleanPath),
			Path:       cleanPath,
			Size:       hdr.Size,
			Mode:       hdr.FileInfo().Mode().String(),
			ModTime:    hdr.ModTime,
			IsDir:      false,
			LinkTarget: hdr.Linkname,
		}

		limit := maxBytes
		if limit <= 0 {
			limit = 5 * 1024 * 1024 // 5MB default
		}

		data, err := io.ReadAll(io.LimitReader(tr, limit))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		return data, fi, nil
	}

	return nil, nil, fmt.Errorf("file %s not found in container", cleanPath)
}

// DownloadContainerFile streams the unpacked file content from filePath inside container.
func (s *Service) DownloadContainerFile(ctx context.Context, id, filePath string) (io.ReadCloser, *FileInfo, error) {
	if s == nil || s.cli == nil {
		return nil, nil, errors.New("docker service not available")
	}

	cleanPath := path.Clean(filePath)
	reader, _, err := s.cli.CopyFromContainer(ctx, id, cleanPath)
	if err != nil {
		return nil, nil, err
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = reader.Close()
			return nil, nil, err
		}

		if hdr.FileInfo().IsDir() {
			_ = reader.Close()
			return nil, nil, fmt.Errorf("cannot download %s: path is a directory", cleanPath)
		}

		fi := &FileInfo{
			Name:       path.Base(cleanPath),
			Path:       cleanPath,
			Size:       hdr.Size,
			Mode:       hdr.FileInfo().Mode().String(),
			ModTime:    hdr.ModTime,
			IsDir:      false,
			LinkTarget: hdr.Linkname,
		}

		return &tarFileReader{r: tr, closer: reader}, fi, nil
	}

	_ = reader.Close()
	return nil, nil, fmt.Errorf("file %s not found in container", cleanPath)
}

// UploadContainerFile packs filename and content into a tar archive and copies it to destDir inside container.
func (s *Service) UploadContainerFile(ctx context.Context, id, destDir, filename string, content io.Reader, size int64, mode os.FileMode) error {
	if s == nil || s.cli == nil {
		return errors.New("docker service not available")
	}

	cleanDir := path.Clean(destDir)
	if cleanDir == "" || cleanDir == "." {
		cleanDir = "/"
	}
	if !strings.HasPrefix(cleanDir, "/") {
		cleanDir = "/" + cleanDir
	}

	cleanFilename := path.Base(filename)
	if cleanFilename == "" || cleanFilename == "." || cleanFilename == "/" {
		return errors.New("invalid filename")
	}

	if mode == 0 {
		mode = 0644
	}

	if size < 0 && content != nil {
		b, err := io.ReadAll(content)
		if err != nil {
			return fmt.Errorf("failed to read content: %w", err)
		}
		size = int64(len(b))
		content = bytes.NewReader(b)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name:     cleanFilename,
		Mode:     int64(mode),
		Size:     size,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if size > 0 && content != nil {
		if _, err := io.CopyN(tw, content, size); err != nil {
			return fmt.Errorf("failed to write tar content: %w", err)
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar archive: %w", err)
	}

	return s.cli.CopyToContainer(ctx, id, cleanDir, &buf, container.CopyToContainerOptions{})
}

// DeleteContainerFile deletes targetPath inside a running container.
// It returns an error if the container is not running.
func (s *Service) DeleteContainerFile(ctx context.Context, id, targetPath string) error {
	if s == nil || s.cli == nil {
		return errors.New("docker service not available")
	}

	cleanPath := path.Clean(targetPath)
	if cleanPath == "" || cleanPath == "." || cleanPath == "/" {
		return errors.New("refusing to delete root directory")
	}

	inspect, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}
	if inspect.State == nil || !inspect.State.Running {
		state := "unknown"
		if inspect.State != nil {
			state = inspect.State.Status
		}
		return fmt.Errorf("cannot delete file: container is not running (state: %s)", state)
	}

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"rm", "-rf", "--", cleanPath},
	}

	execResp, err := s.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create delete exec: %w", err)
	}

	attachResp, err := s.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to delete exec: %w", err)
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)

	inspectExec, err := s.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect delete exec: %w", err)
	}
	if inspectExec.ExitCode != 0 {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("exit code %d", inspectExec.ExitCode)
		}
		return fmt.Errorf("delete failed: %s", errMsg)
	}

	return nil
}
