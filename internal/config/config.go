package config

import (
	"flag"
	"os"
	"strconv"
)

var Version = "0.2.0"

type Config struct {
	Host        string
	Port        int
	SocketPath  string
	DataDir     string
	NoAuth      bool
	JWTSecret   string
	ShowVersion bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Host:       getEnv("PDSWARM_HOST", "0.0.0.0"),
		Port:       getEnvInt("PDSWARM_PORT", 9000),
		SocketPath: getEnv("DOCKER_HOST", "/var/run/docker.sock"),
		DataDir:    getEnv("PDSWARM_DATA_DIR", "./data"),
		NoAuth:     getEnvBool("PDSWARM_NO_AUTH", false),
		JWTSecret:  getEnv("PDSWARM_JWT_SECRET", ""),
	}

	flag.StringVar(&cfg.Host, "host", cfg.Host, "Host to bind HTTP server")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Port to bind HTTP server")
	flag.StringVar(&cfg.SocketPath, "socket", cfg.SocketPath, "Docker daemon socket or endpoint")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Data directory for bbolt database")
	flag.BoolVar(&cfg.NoAuth, "no-auth", cfg.NoAuth, "Disable authentication checks")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT signing secret (generated if empty)")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	flag.BoolVar(&cfg.ShowVersion, "v", false, "Print version and exit (shorthand)")

	if !flag.Parsed() {
		flag.Parse()
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
