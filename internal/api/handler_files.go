package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
)

// HandleListContainerFiles lists files in a container directory.
func HandleListContainerFiles(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		dirPath := r.URL.Query().Get("path")
		if dirPath == "" {
			dirPath = "/"
		}

		files, err := ds.ListContainerFiles(r.Context(), id, dirPath)
		if err != nil {
			if client.IsErrNotFound(err) || isNotFoundError(err) {
				respondJSONError(w, http.StatusNotFound, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to list container files: "+err.Error())
			return
		}

		if files == nil {
			files = []docker.FileInfo{}
		}

		respondJSON(w, http.StatusOK, files)
	}
}

// ReadFileResponse represents the JSON response for reading a text file.
type ReadFileResponse struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Content string `json:"content"`
}

// HandleReadContainerFile reads the content of a container file up to max bytes.
func HandleReadContainerFile(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		filePath := r.URL.Query().Get("path")
		if strings.TrimSpace(filePath) == "" {
			respondJSONError(w, http.StatusBadRequest, "path query parameter is required")
			return
		}

		var maxBytes int64
		if maxStr := r.URL.Query().Get("max"); maxStr != "" {
			maxBytes, _ = strconv.ParseInt(maxStr, 10, 64)
		}

		content, fi, err := ds.ReadContainerFile(r.Context(), id, filePath, maxBytes)
		if err != nil {
			if client.IsErrNotFound(err) || isNotFoundError(err) {
				respondJSONError(w, http.StatusNotFound, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to read container file: "+err.Error())
			return
		}

		if !utf8.Valid(content) || bytes.IndexByte(content, 0) != -1 {
			respondJSONError(w, http.StatusBadRequest, "file contains binary data and cannot be displayed as text")
			return
		}

		respondJSON(w, http.StatusOK, ReadFileResponse{
			Name:    fi.Name,
			Path:    fi.Path,
			Size:    fi.Size,
			Content: string(content),
		})
	}
}

// HandleDownloadContainerFile streams a file from the container as an octet-stream attachment.
func HandleDownloadContainerFile(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		filePath := r.URL.Query().Get("path")
		if strings.TrimSpace(filePath) == "" {
			respondJSONError(w, http.StatusBadRequest, "path query parameter is required")
			return
		}

		reader, fi, err := ds.DownloadContainerFile(r.Context(), id, filePath)
		if err != nil {
			if client.IsErrNotFound(err) || isNotFoundError(err) {
				respondJSONError(w, http.StatusNotFound, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to download container file: "+err.Error())
			return
		}
		defer reader.Close()

		filename := fi.Name
		if filename == "" {
			filename = path.Base(filePath)
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		if fi.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(fi.Size, 10))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, reader)
	}
}

// HandleUploadContainerFile uploads a file into the container via multipart form or raw body.
func HandleUploadContainerFile(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		destDir := r.URL.Query().Get("path")
		if destDir == "" {
			destDir = "/"
		}

		var filename string
		var content io.Reader
		var size int64 = -1

		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				respondJSONError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				respondJSONError(w, http.StatusBadRequest, "missing 'file' field in multipart form: "+err.Error())
				return
			}
			defer file.Close()
			filename = header.Filename
			size = header.Size
			content = file
		} else {
			filename = r.URL.Query().Get("filename")
			if strings.TrimSpace(filename) == "" {
				respondJSONError(w, http.StatusBadRequest, "filename query parameter is required for direct upload")
				return
			}
			size = r.ContentLength
			// Enforce a safe upper bound (500MB) on direct stream uploads to prevent memory exhaustion
			r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
			content = r.Body
		}

		if err := ds.UploadContainerFile(r.Context(), id, destDir, filename, content, size, 0644); err != nil {
			if client.IsErrNotFound(err) || isNotFoundError(err) {
				respondJSONError(w, http.StatusNotFound, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to upload file: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "uploaded",
			"filename": filename,
			"path":     path.Join(destDir, filename),
		})
	}
}

// HandleDeleteContainerFile deletes a file or directory inside a running container.
func HandleDeleteContainerFile(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		targetPath := r.URL.Query().Get("path")
		if strings.TrimSpace(targetPath) == "" {
			respondJSONError(w, http.StatusBadRequest, "path query parameter is required")
			return
		}

		if err := ds.DeleteContainerFile(r.Context(), id, targetPath); err != nil {
			if client.IsErrNotFound(err) || isNotFoundError(err) {
				respondJSONError(w, http.StatusNotFound, err.Error())
				return
			}
			if strings.Contains(err.Error(), "not running") || strings.Contains(err.Error(), "refusing to delete") {
				respondJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to delete file: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"status": "deleted",
			"path":   targetPath,
		})
	}
}

// HandleStatContainerFile returns path stat information for a container path.
func HandleStatContainerFile(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "container id is required")
			return
		}

		targetPath := r.URL.Query().Get("path")
		if strings.TrimSpace(targetPath) == "" {
			respondJSONError(w, http.StatusBadRequest, "path query parameter is required")
			return
		}

		stat, err := ds.StatContainerPath(r.Context(), id, targetPath)
		if err != nil {
			if client.IsErrNotFound(err) || isNotFoundError(err) {
				respondJSONError(w, http.StatusNotFound, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to stat container path: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, stat)
	}
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such container") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no such file or directory")
}
