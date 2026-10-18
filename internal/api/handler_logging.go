package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

// LoggingConfigDTO represents Docker/Podman logging and rotation configuration.
type LoggingConfigDTO struct {
	LogDriver        string `json:"log_driver"`
	MaxSize          string `json:"max_size"`
	MaxFile          string `json:"max_file"`
	Compress         bool   `json:"compress"`
	ConfigFile       string `json:"config_file"`
	ConfigWritable   bool   `json:"config_writable"`
	DaemonConfigured bool   `json:"daemon_configured"`
	GeneratedSnippet string `json:"generated_snippet"`
	ApplyCommand     string `json:"apply_command"`
	EngineType       string `json:"engine_type"`
}

// UpdateLoggingConfigRequest payload for modifying log settings.
type UpdateLoggingConfigRequest struct {
	LogDriver       string `json:"log_driver"`
	MaxSize         string `json:"max_size"`
	MaxFile         string `json:"max_file"`
	Compress        bool   `json:"compress"`
	WriteDaemonFile bool   `json:"write_daemon_file"`
}

// Helper to determine daemon config file path and check writeability
func getDaemonConfigPath(engineType string) (string, bool) {
	var path string
	if engineType == "podman" {
		homeDir, err := os.UserHomeDir()
		if err == nil && homeDir != "" {
			path = filepath.Join(homeDir, ".config", "containers", "containers.conf")
		} else {
			path = "/etc/containers/containers.conf"
		}
	} else {
		path = "/etc/docker/daemon.json"
	}

	writable := isFileOrDirWritable(path)
	return path, writable
}

func isFileOrDirWritable(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		// File exists, check if writable
		f, err := os.OpenFile(path, os.O_WRONLY, 0644)
		if err == nil {
			f.Close()
			return true
		}
		return false
	}

	// File doesn't exist, check if directory is writable
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".pdswarm_test_write")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.Close()
		os.Remove(testFile)
		return true
	}
	return false
}

// Generate merged daemon.json snippet preserving existing attributes
func generateDockerDaemonSnippet(existingContent []byte, logDriver, maxSize, maxFile string, compress bool) (string, bool) {
	data := make(map[string]interface{})
	if len(existingContent) > 0 {
		_ = json.Unmarshal(existingContent, &data)
	}

	// Check if log-opts or log-driver was already configured
	isConfigured := false
	if _, ok := data["log-driver"]; ok {
		isConfigured = true
	}
	if _, ok := data["log-opts"]; ok {
		isConfigured = true
	}

	data["log-driver"] = logDriver
	logOpts := map[string]interface{}{
		"max-size": maxSize,
		"max-file": maxFile,
	}
	if compress {
		logOpts["compress"] = "true"
	}
	data["log-opts"] = logOpts

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "{}", isConfigured
	}
	return string(out), isConfigured
}

// HandleGetLoggingConfig returns the current logging options and daemon status.
func HandleGetLoggingConfig(ds *docker.Service, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		engine := ds.GetEngineInfo(r.Context())
		engineType := engine.Type

		logDriver := "json-file"
		if val, err := st.GetSetting("log_driver"); err == nil && val != "" {
			logDriver = val
		}

		maxSize := "10m"
		if val, err := st.GetSetting("log_max_size"); err == nil && val != "" {
			maxSize = val
		}

		maxFile := "3"
		if val, err := st.GetSetting("log_max_file"); err == nil && val != "" {
			maxFile = val
		}

		compress := true
		if val, err := st.GetSetting("log_compress"); err == nil && val != "" {
			compress = val == "true"
		}

		configPath, writable := getDaemonConfigPath(engineType)
		var existingBytes []byte
		if b, err := os.ReadFile(configPath); err == nil {
			existingBytes = b
		}

		var snippet string
		var daemonConfigured bool
		var applyCmd string

		if engineType == "podman" {
			snippet = fmt.Sprintf("[engine]\nlog_driver = \"%s\"\nlog_size_max = %s\n", logDriver, maxSize)
			applyCmd = fmt.Sprintf("mkdir -p $(dirname %s) && tee %s << 'EOF'\n%sEOF\nsystemctl --user restart podman", configPath, configPath, snippet)
			daemonConfigured = strings.Contains(string(existingBytes), "log_size_max") || strings.Contains(string(existingBytes), "log_driver")
		} else {
			snippet, daemonConfigured = generateDockerDaemonSnippet(existingBytes, logDriver, maxSize, maxFile, compress)
			applyCmd = fmt.Sprintf("sudo tee %s << 'EOF'\n%s\nEOF\nsudo systemctl reload docker", configPath, snippet)
		}

		dto := LoggingConfigDTO{
			LogDriver:        logDriver,
			MaxSize:          maxSize,
			MaxFile:          maxFile,
			Compress:         compress,
			ConfigFile:       configPath,
			ConfigWritable:   writable,
			DaemonConfigured: daemonConfigured,
			GeneratedSnippet: snippet,
			ApplyCommand:     applyCmd,
			EngineType:       engineType,
		}

		respondJSON(w, http.StatusOK, dto)
	}
}

// HandleUpdateLoggingConfig saves logging settings and optionally writes daemon config if permitted.
func HandleUpdateLoggingConfig(ds *docker.Service, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var req UpdateLoggingConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if req.LogDriver == "" {
			req.LogDriver = "json-file"
		}
		if req.MaxSize == "" {
			req.MaxSize = "10m"
		}
		if req.MaxFile == "" {
			req.MaxFile = "3"
		}

		// Save in store
		_ = st.SaveSetting("log_driver", req.LogDriver)
		_ = st.SaveSetting("log_max_size", req.MaxSize)
		_ = st.SaveSetting("log_max_file", req.MaxFile)
		_ = st.SaveSetting("log_compress", strconv.FormatBool(req.Compress))

		engine := ds.GetEngineInfo(r.Context())
		engineType := engine.Type
		configPath, writable := getDaemonConfigPath(engineType)

		var existingBytes []byte
		if b, err := os.ReadFile(configPath); err == nil {
			existingBytes = b
		}

		snippet, daemonConfigured := generateDockerDaemonSnippet(existingBytes, req.LogDriver, req.MaxSize, req.MaxFile, req.Compress)
		applyCmd := fmt.Sprintf("sudo tee %s << 'EOF'\n%s\nEOF\nsudo systemctl reload docker", configPath, snippet)

		if engineType == "podman" {
			snippet = fmt.Sprintf("[engine]\nlog_driver = \"%s\"\nlog_size_max = %s\n", req.LogDriver, req.MaxSize)
			applyCmd = fmt.Sprintf("mkdir -p $(dirname %s) && tee %s << 'EOF'\n%sEOF\nsystemctl --user restart podman", configPath, configPath, snippet)
		}

		if req.WriteDaemonFile && writable {
			_ = os.MkdirAll(filepath.Dir(configPath), 0755)
			_ = os.WriteFile(configPath, []byte(snippet), 0644)
			daemonConfigured = true
		}

		dto := LoggingConfigDTO{
			LogDriver:        req.LogDriver,
			MaxSize:          req.MaxSize,
			MaxFile:          req.MaxFile,
			Compress:         req.Compress,
			ConfigFile:       configPath,
			ConfigWritable:   writable,
			DaemonConfigured: daemonConfigured,
			GeneratedSnippet: snippet,
			ApplyCommand:     applyCmd,
			EngineType:       engineType,
		}

		respondJSON(w, http.StatusOK, dto)
	}
}
