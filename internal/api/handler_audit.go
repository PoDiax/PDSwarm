package api

import (
	"net/http"

	"github.com/podiax/pdswarm/internal/audit"
	"github.com/podiax/pdswarm/internal/config"
)

type AuditLogsResponse struct {
	Path           string   `json:"path"`
	Level          string   `json:"level"`
	Entries        []string `json:"entries"`
	Fail2banFilter string   `json:"fail2ban_filter"`
	Fail2banJail   string   `json:"fail2ban_jail"`
}

// HandleGetAuditLogs returns recent audit log entries and copyable Fail2ban configuration.
func HandleGetAuditLogs(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := audit.Default()

		entries, err := logger.ReadRecent(100)
		if err != nil {
			entries = []string{"Error reading audit log: " + err.Error()}
		}

		path := ""
		level := "auth"
		if cfg != nil {
			path = cfg.AuthLogPath
			level = cfg.LogLevel
		} else if logger != nil {
			path = logger.Path()
			level = string(logger.Level())
		}

		resp := AuditLogsResponse{
			Path:           path,
			Level:          level,
			Entries:        entries,
			Fail2banFilter: logger.Fail2banFilterConfig(),
			Fail2banJail:   logger.Fail2banJailConfig(),
		}

		respondJSON(w, http.StatusOK, resp)
	}
}
