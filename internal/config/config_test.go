package config_test

import (
	"flag"
	"os"
	"testing"

	"github.com/podiax/pdswarm/internal/config"
)

func testConfigDefaults(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{os.Args[0]}

	// Reset flags for test isolation
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Host)
	}
	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
	if cfg.SocketPath != "/var/run/docker.sock" {
		t.Errorf("expected socket /var/run/docker.sock, got %s", cfg.SocketPath)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("expected data dir ./data, got %s", cfg.DataDir)
	}
	if cfg.NoAuth != false {
		t.Errorf("expected no-auth false, got %v", cfg.NoAuth)
	}
	if cfg.JWTSecret != "" {
		t.Errorf("expected empty jwt secret, got %s", cfg.JWTSecret)
	}
}

func testConfigEnvVars(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{os.Args[0]}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	os.Setenv("PDSWARM_HOST", "127.0.0.1")
	os.Setenv("PDSWARM_PORT", "8080")
	os.Setenv("DOCKER_HOST", "unix:///custom/docker.sock")
	os.Setenv("PDSWARM_DATA_DIR", "/var/lib/pdswarm")
	os.Setenv("PDSWARM_NO_AUTH", "true")
	os.Setenv("PDSWARM_JWT_SECRET", "supersecret")
	defer func() {
		os.Unsetenv("PDSWARM_HOST")
		os.Unsetenv("PDSWARM_PORT")
		os.Unsetenv("DOCKER_HOST")
		os.Unsetenv("PDSWARM_DATA_DIR")
		os.Unsetenv("PDSWARM_NO_AUTH")
		os.Unsetenv("PDSWARM_JWT_SECRET")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.SocketPath != "unix:///custom/docker.sock" {
		t.Errorf("expected socket unix:///custom/docker.sock, got %s", cfg.SocketPath)
	}
	if cfg.DataDir != "/var/lib/pdswarm" {
		t.Errorf("expected data dir /var/lib/pdswarm, got %s", cfg.DataDir)
	}
	if cfg.NoAuth != true {
		t.Errorf("expected no-auth true, got %v", cfg.NoAuth)
	}
	if cfg.JWTSecret != "supersecret" {
		t.Errorf("expected jwt secret supersecret, got %s", cfg.JWTSecret)
	}
}

func testConfigFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{
		"pdswarm",
		"-host", "192.168.1.100",
		"-port", "7000",
		"-socket", "tcp://192.168.1.10:2375",
		"-data-dir", "/tmp/data",
		"-no-auth",
		"-jwt-secret", "flagsecret",
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %s", cfg.Host)
	}
	if cfg.Port != 7000 {
		t.Errorf("expected port 7000, got %d", cfg.Port)
	}
	if cfg.SocketPath != "tcp://192.168.1.10:2375" {
		t.Errorf("expected socket tcp://192.168.1.10:2375, got %s", cfg.SocketPath)
	}
	if cfg.DataDir != "/tmp/data" {
		t.Errorf("expected data dir /tmp/data, got %s", cfg.DataDir)
	}
	if cfg.NoAuth != true {
		t.Errorf("expected no-auth true, got %v", cfg.NoAuth)
	}
	if cfg.JWTSecret != "flagsecret" {
		t.Errorf("expected jwt secret flagsecret, got %s", cfg.JWTSecret)
	}
}

func testConfigVersion(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"pdswarm", "-version"}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.ShowVersion {
		t.Errorf("expected ShowVersion true, got false")
	}
}

func TestConfig(t *testing.T) {
	t.Run("defaults", testConfigDefaults)
	t.Run("env_vars", testConfigEnvVars)
	t.Run("flags", testConfigFlags)
	t.Run("version", testConfigVersion)
}
