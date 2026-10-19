package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/store"
)

// UpdateStatusDTO represents current version, latest available release, and auto-update configuration.
type UpdateStatusDTO struct {
	CurrentVersion      string `json:"current_version"`
	LatestVersion       string `json:"latest_version"`
	UpdateAvailable     bool   `json:"update_available"`
	ReleaseURL          string `json:"release_url,omitempty"`
	ReleaseNotes        string `json:"release_notes,omitempty"`
	PublishedAt         string `json:"published_at,omitempty"`
	CheckedAt           string `json:"checked_at"`
	AutoUpdateEnabled   bool   `json:"auto_update_enabled"`
	AutoUpdateSupported bool   `json:"auto_update_supported"`
	InstallType         string `json:"install_type"` // "systemd", "docker", "standalone"
	TimerActive         bool   `json:"timer_active"`
	Message             string `json:"message,omitempty"`
}

// SetAutoUpdateRequest payload for enabling or disabling background auto updates.
type SetAutoUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

// ApplyUpdateResponse payload when triggering an update.
type ApplyUpdateResponse struct {
	Status  string `json:"status"` // "initiated", "manual", "up_to_date", "error"
	Message string `json:"message"`
}

type releaseCache struct {
	sync.RWMutex
	lastChecked    time.Time
	latestVersion  string
	releaseURL     string
	releaseNotes   string
	publishedAt    string
	fetchError     string
}

var globalReleaseCache = &releaseCache{}

// compareVersions compares two semver strings (e.g. "0.2.0" vs "v0.2.1").
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func compareVersions(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.TrimSpace(v1), "v")
	clean2 := strings.TrimPrefix(strings.TrimSpace(v2), "v")

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}

func fetchLatestRelease(ctx context.Context, force bool) (string, string, string, string, error) {
	globalReleaseCache.RLock()
	if !force && time.Since(globalReleaseCache.lastChecked) < 1*time.Hour && globalReleaseCache.latestVersion != "" {
		v, u, n, p := globalReleaseCache.latestVersion, globalReleaseCache.releaseURL, globalReleaseCache.releaseNotes, globalReleaseCache.publishedAt
		globalReleaseCache.RUnlock()
		return v, u, n, p, nil
	}
	globalReleaseCache.RUnlock()

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/PoDiax/PDSwarm/releases/latest", nil)
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("User-Agent", "PDSwarm/"+config.Version)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", "", fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(body))
	}

	var ghRelease struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return "", "", "", "", err
	}

	globalReleaseCache.Lock()
	globalReleaseCache.lastChecked = time.Now()
	globalReleaseCache.latestVersion = ghRelease.TagName
	globalReleaseCache.releaseURL = ghRelease.HTMLURL
	globalReleaseCache.releaseNotes = ghRelease.Body
	globalReleaseCache.publishedAt = ghRelease.PublishedAt
	globalReleaseCache.fetchError = ""
	globalReleaseCache.Unlock()

	return ghRelease.TagName, ghRelease.HTMLURL, ghRelease.Body, ghRelease.PublishedAt, nil
}

func detectInstallType() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return "docker"
	}
	if os.Getenv("PDSWARM_IN_DOCKER") == "true" {
		return "docker"
	}

	if os.Getenv("INVOCATION_ID") != "" || os.Getenv("JOURNAL_STREAM") != "" {
		return "systemd"
	}
	if _, err := os.Stat("/etc/systemd/system/pdswarm.service"); err == nil {
		return "systemd"
	}
	if _, err := os.Stat("/etc/systemd/system/pdswarm-update.timer"); err == nil {
		return "systemd"
	}

	return "standalone"
}

func checkAutoUpdateStatus(st store.Store, installType string) (enabled bool, active bool, supported bool) {
	if installType == "docker" {
		return false, false, false
	}

	supported = true

	if _, err := exec.LookPath("systemctl"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "systemctl", "--no-ask-password", "is-enabled", "pdswarm-update.timer").CombinedOutput()
		outStr := strings.TrimSpace(string(out))
		if err == nil && strings.Contains(outStr, "enabled") {
			enabled = true
		} else if strings.Contains(outStr, "disabled") {
			enabled = false
		} else {
			if st != nil {
				if val, err := st.GetSetting("auto_update_enabled"); err == nil {
					enabled = val == "true"
				}
			}
		}

		ctxActive, cancelActive := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelActive()
		outActive, err := exec.CommandContext(ctxActive, "systemctl", "--no-ask-password", "is-active", "pdswarm-update.timer").CombinedOutput()
		if err == nil && strings.TrimSpace(string(outActive)) == "active" {
			active = true
		}
		return enabled, active, supported
	}

	if st != nil {
		if val, err := st.GetSetting("auto_update_enabled"); err == nil {
			enabled = val == "true"
		}
	}
	return enabled, false, false
}

func ensureSystemdUpdateUnits() error {
	serviceContent := `[Unit]
Description=PDSwarm Auto-Updater Service
Documentation=https://github.com/podiax/pdswarm
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/pdswarm-update
`

	timerContent := `[Unit]
Description=Daily Auto-Update Check for PDSwarm
Documentation=https://github.com/podiax/pdswarm

[Timer]
OnCalendar=*-*-* 04:00:00
RandomizedDelaySec=1800
Persistent=true

[Install]
WantedBy=timers.target
`

	_ = os.WriteFile("/etc/systemd/system/pdswarm-update.service", []byte(serviceContent), 0644)
	_ = os.WriteFile("/etc/systemd/system/pdswarm-update.timer", []byte(timerContent), 0644)

	// Ensure updater script exists
	updaterPath := "/usr/local/bin/pdswarm-update"
	if _, err := os.Stat(updaterPath); os.IsNotExist(err) {
		updaterScript := `#!/usr/bin/env bash
set -euo pipefail

REPO="PoDiax/PDSwarm"
BIN_PATH="/usr/local/bin/pdswarm"
SERVICE_NAME="pdswarm.service"

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) TARGET_ARCH="amd64" ;;
  aarch64|arm64) TARGET_ARCH="arm64" ;;
  *) exit 1 ;;
esac

RELEASE_API_URL="https://api.github.com/repos/${REPO}/releases/latest"
RELEASE_JSON="$(curl -fsSL -H "User-Agent: PDSwarm-Updater" "${RELEASE_API_URL}" 2>/dev/null || true)"
if [[ -z "${RELEASE_JSON}" ]]; then exit 0; fi

LATEST_TAG="$(echo "${RELEASE_JSON}" | grep -o '"tag_name": *"[^"]*"' | head -n1 | cut -d'"' -f4 || true)"
DOWNLOAD_URL="$(echo "${RELEASE_JSON}" | grep -o "https://[^\"]*/releases/download/[^\"]*/pdswarm-linux-${TARGET_ARCH}" | head -n1 || true)"
if [[ -z "${DOWNLOAD_URL}" ]]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/pdswarm-linux-${TARGET_ARCH}"
fi

TMP_BIN="${BIN_PATH}.tmp.$$"
if curl -fsSL -H "User-Agent: PDSwarm-Updater" -o "${TMP_BIN}" "${DOWNLOAD_URL}"; then
  chmod +x "${TMP_BIN}"
  if "${TMP_BIN}" -version 2>&1 | grep -q "PDSwarm"; then
    mv -f "${TMP_BIN}" "${BIN_PATH}"
    systemctl restart "${SERVICE_NAME}" 2>/dev/null || true
  else
    rm -f "${TMP_BIN}"
  fi
fi
`
		_ = os.WriteFile(updaterPath, []byte(updaterScript), 0755)
	}

	return nil
}

// HandleGetUpdateStatus returns current version, latest GitHub release, and auto-update configuration.
func HandleGetUpdateStatus(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		forceCheck := r.URL.Query().Get("check") == "true"
		latestVer, relURL, relNotes, publishedAt, err := fetchLatestRelease(r.Context(), forceCheck)

		curVer := config.Version
		if !strings.HasPrefix(curVer, "v") {
			curVer = "v" + curVer
		}

		updateAvail := false
		if err == nil && latestVer != "" {
			updateAvail = compareVersions(curVer, latestVer) < 0
		} else if latestVer == "" {
			latestVer = curVer
		}

		installType := detectInstallType()
		autoEnabled, timerActive, autoSupported := checkAutoUpdateStatus(st, installType)

		msg := ""
		if err != nil {
			msg = "Note: Could not check GitHub releases: " + err.Error()
		}

		dto := UpdateStatusDTO{
			CurrentVersion:      curVer,
			LatestVersion:       latestVer,
			UpdateAvailable:     updateAvail,
			ReleaseURL:          relURL,
			ReleaseNotes:        relNotes,
			PublishedAt:         publishedAt,
			CheckedAt:           time.Now().UTC().Format(time.RFC3339),
			AutoUpdateEnabled:   autoEnabled,
			AutoUpdateSupported: autoSupported,
			InstallType:         installType,
			TimerActive:         timerActive,
			Message:             msg,
		}

		respondJSON(w, http.StatusOK, dto)
	}
}

// HandleCheckUpdate explicitly queries GitHub Releases for updates.
func HandleCheckUpdate(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latestVer, relURL, relNotes, publishedAt, err := fetchLatestRelease(r.Context(), true)

		curVer := config.Version
		if !strings.HasPrefix(curVer, "v") {
			curVer = "v" + curVer
		}

		updateAvail := false
		if err == nil && latestVer != "" {
			updateAvail = compareVersions(curVer, latestVer) < 0
		} else if latestVer == "" {
			latestVer = curVer
		}

		installType := detectInstallType()
		autoEnabled, timerActive, autoSupported := checkAutoUpdateStatus(st, installType)

		msg := ""
		if err != nil {
			msg = "Failed to query GitHub API: " + err.Error()
		}

		dto := UpdateStatusDTO{
			CurrentVersion:      curVer,
			LatestVersion:       latestVer,
			UpdateAvailable:     updateAvail,
			ReleaseURL:          relURL,
			ReleaseNotes:        relNotes,
			PublishedAt:         publishedAt,
			CheckedAt:           time.Now().UTC().Format(time.RFC3339),
			AutoUpdateEnabled:   autoEnabled,
			AutoUpdateSupported: autoSupported,
			InstallType:         installType,
			TimerActive:         timerActive,
			Message:             msg,
		}

		respondJSON(w, http.StatusOK, dto)
	}
}

// HandleSetAutoUpdate toggles the background auto-update systemd timer.
func HandleSetAutoUpdate(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SetAutoUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if st != nil {
			_ = st.SaveSetting("auto_update_enabled", strconv.FormatBool(req.Enabled))
		}

		installType := detectInstallType()
		var actionErr error

		if installType == "systemd" || installType == "standalone" {
			if _, err := exec.LookPath("systemctl"); err == nil && os.Geteuid() == 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if req.Enabled {
					_ = ensureSystemdUpdateUnits()
					_ = exec.CommandContext(ctx, "systemctl", "--no-ask-password", "daemon-reload").Run()
					actionErr = exec.CommandContext(ctx, "systemctl", "--no-ask-password", "enable", "--now", "pdswarm-update.timer").Run()
				} else {
					actionErr = exec.CommandContext(ctx, "systemctl", "--no-ask-password", "disable", "--now", "pdswarm-update.timer").Run()
				}
			}
		}

		autoEnabled, timerActive, autoSupported := checkAutoUpdateStatus(st, installType)

		curVer := config.Version
		if !strings.HasPrefix(curVer, "v") {
			curVer = "v" + curVer
		}

		msg := ""
		if actionErr != nil {
			msg = fmt.Sprintf("Saved preference, but systemctl returned: %v", actionErr)
		} else if req.Enabled {
			msg = "Automated daily updates enabled (daily at 04:00 AM)."
		} else {
			msg = "Automated daily updates disabled."
		}

		dto := UpdateStatusDTO{
			CurrentVersion:      curVer,
			LatestVersion:       globalReleaseCache.latestVersion,
			UpdateAvailable:     compareVersions(curVer, globalReleaseCache.latestVersion) < 0,
			ReleaseURL:          globalReleaseCache.releaseURL,
			ReleaseNotes:        globalReleaseCache.releaseNotes,
			PublishedAt:         globalReleaseCache.publishedAt,
			CheckedAt:           time.Now().UTC().Format(time.RFC3339),
			AutoUpdateEnabled:   autoEnabled,
			AutoUpdateSupported: autoSupported,
			InstallType:         installType,
			TimerActive:         timerActive,
			Message:             msg,
		}

		respondJSON(w, http.StatusOK, dto)
	}
}

// HandleApplyUpdate initiates an immediate update of PDSwarm.
func HandleApplyUpdate(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		installType := detectInstallType()

		if installType == "docker" {
			respondJSON(w, http.StatusOK, ApplyUpdateResponse{
				Status:  "manual",
				Message: "PDSwarm is running inside Docker. Please run 'docker compose pull && docker compose up -d' or use Watchtower to update the container.",
			})
			return
		}

		updaterPath := "/usr/local/bin/pdswarm-update"
		if _, err := os.Stat(updaterPath); err == nil {
			cmd := exec.Command(updaterPath, "--force")
			if err := cmd.Start(); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to execute updater: "+err.Error())
				return
			}
			respondJSON(w, http.StatusOK, ApplyUpdateResponse{
				Status:  "initiated",
				Message: "Update initiated via pdswarm-update. The service will restart automatically once installation completes.",
			})
			return
		}

		// Fallback in-process updater
		go func() {
			time.Sleep(1 * time.Second)
			_ = runDirectBinaryUpdate()
		}()

		respondJSON(w, http.StatusOK, ApplyUpdateResponse{
			Status:  "initiated",
			Message: "Update initiated. PDSwarm will download the latest binary and restart shortly.",
		})
	}
}

func runDirectBinaryUpdate() error {
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported arch %s", arch)
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "/usr/local/bin/pdswarm"
	}
	execPath, _ = filepath.EvalSymlinks(execPath)

	client := &http.Client{Timeout: 60 * time.Second}
	url := fmt.Sprintf("https://github.com/PoDiax/PDSwarm/releases/latest/download/pdswarm-linux-%s", arch)

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile := execPath + ".tmp." + strconv.FormatInt(time.Now().Unix(), 10)
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpFile)
		return err
	}

	// Verify binary executes
	cmd := exec.Command(tmpFile, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "PDSwarm") {
		os.Remove(tmpFile)
		return fmt.Errorf("downloaded file failed verification")
	}

	if err := os.Rename(tmpFile, execPath); err != nil {
		os.Remove(tmpFile)
		return err
	}

	// Trigger restart
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.Command("systemctl", "restart", "pdswarm.service").Run()
	} else {
		os.Exit(0)
	}

	return nil
}
