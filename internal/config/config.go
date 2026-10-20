package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var Version = "0.2.5"

type Config struct {
	Host           string
	Port           int
	SocketPath     string
	DataDir        string
	NoAuth         bool
	JWTSecret      string
	ShowVersion    bool
	AuthLogPath    string
	LogLevel       string
	TrustedProxies []string
	AllowedOrigins []string
}

func Load() (*Config, error) {
	defaultProxies := "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128"
	trustedProxiesRaw := getEnv("PDSWARM_TRUSTED_PROXIES", defaultProxies)
	allowedOriginsRaw := getEnv("PDSWARM_ALLOWED_ORIGINS", "")

	cfg := &Config{
		Host:        getEnv("PDSWARM_HOST", "0.0.0.0"),
		Port:        getEnvInt("PDSWARM_PORT", 9000),
		SocketPath:  getEnv("DOCKER_HOST", "/var/run/docker.sock"),
		DataDir:     getEnv("PDSWARM_DATA_DIR", "./data"),
		NoAuth:      getEnvBool("PDSWARM_NO_AUTH", false),
		JWTSecret:   getEnv("PDSWARM_JWT_SECRET", ""),
		AuthLogPath: getEnv("PDSWARM_AUTH_LOG", ""),
		LogLevel:    getEnv("PDSWARM_LOG_LEVEL", "auth"),
	}

	flag.StringVar(&cfg.Host, "host", cfg.Host, "Host to bind HTTP server")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Port to bind HTTP server")
	flag.StringVar(&cfg.SocketPath, "socket", cfg.SocketPath, "Docker daemon socket or endpoint")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Data directory for bbolt database")
	flag.BoolVar(&cfg.NoAuth, "no-auth", cfg.NoAuth, "Disable authentication checks")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT signing secret (generated if empty)")
	flag.StringVar(&cfg.AuthLogPath, "auth-log", cfg.AuthLogPath, "Path to security/auth audit log file for fail2ban / jails")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Audit log level: auth, failures_only, all, or off")
	flag.StringVar(&trustedProxiesRaw, "trusted-proxies", trustedProxiesRaw, "Comma-separated list of trusted proxy CIDRs/IPs")
	flag.StringVar(&allowedOriginsRaw, "allowed-origins", allowedOriginsRaw, "Comma-separated list of allowed CORS/WebSocket origins (empty = same-origin only)")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	flag.BoolVar(&cfg.ShowVersion, "v", false, "Print version and exit (shorthand)")

	if !flag.Parsed() {
		flag.Parse()
	}

	if cfg.AuthLogPath == "" {
		cfg.AuthLogPath = filepath.Join(cfg.DataDir, "auth.log")
	}

	for _, p := range strings.Split(trustedProxiesRaw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			cfg.TrustedProxies = append(cfg.TrustedProxies, p)
		}
	}

	for _, o := range strings.Split(allowedOriginsRaw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return fallback
}
