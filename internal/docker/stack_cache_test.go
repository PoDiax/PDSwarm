package docker_test

import (
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestStackDetailCache_GetSet(t *testing.T) {
	cache := docker.NewStackDetailCache(10 * time.Minute)

	_, found := cache.Get("nonexistent")
	if found {
		t.Fatalf("expected nonexistent key to return false")
	}

	cache.Set("mystack", docker.CachedStackDetails{
		ComposeYAML: "version: '3'\nservices:\n  web:\n    image: nginx",
		Env:         "FOO=bar",
		ConfigFile:  "/opt/stacks/mystack/docker-compose.yml",
		WorkingDir:  "/opt/stacks/mystack",
	})

	details, found := cache.Get("mystack")
	if !found {
		t.Fatalf("expected mystack to be found in cache")
	}
	if details.ComposeYAML == "" || details.Env != "FOO=bar" {
		t.Errorf("unexpected cached details: %+v", details)
	}
}

func TestStackDetailCache_Expiration(t *testing.T) {
	cache := docker.NewStackDetailCache(10 * time.Millisecond)
	cache.Set("expiring", docker.CachedStackDetails{ComposeYAML: "services: {}"})

	time.Sleep(25 * time.Millisecond)

	_, found := cache.Get("expiring")
	if found {
		t.Fatalf("expected cache item to expire")
	}
}

func TestStackDetailCache_Invalidate(t *testing.T) {
	cache := docker.NewStackDetailCache(10 * time.Minute)
	cache.Set("to_delete", docker.CachedStackDetails{ComposeYAML: "services: {}"})
	cache.Invalidate("to_delete")

	_, found := cache.Get("to_delete")
	if found {
		t.Fatalf("expected invalidated item to not be found")
	}
}
