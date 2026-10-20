package docker_test

import (
	"testing"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestParseEnvFile(t *testing.T) {
	raw := `
# Configuration file
PUID=1000
PGID=1000
DB_PASSWORD="super secret password"
DB_USERNAME='immich_user'
export UPLOAD_LOCATION=/var/lib/immich
EMPTY_VAR=
# Comment line with = sign
`

	env := docker.ParseEnvFile(raw)

	if env["PUID"] != "1000" {
		t.Errorf("expected PUID=1000, got %q", env["PUID"])
	}
	if env["PGID"] != "1000" {
		t.Errorf("expected PGID=1000, got %q", env["PGID"])
	}
	if env["DB_PASSWORD"] != "super secret password" {
		t.Errorf("expected DB_PASSWORD='super secret password', got %q", env["DB_PASSWORD"])
	}
	if env["DB_USERNAME"] != "immich_user" {
		t.Errorf("expected DB_USERNAME='immich_user', got %q", env["DB_USERNAME"])
	}
	if env["UPLOAD_LOCATION"] != "/var/lib/immich" {
		t.Errorf("expected UPLOAD_LOCATION='/var/lib/immich', got %q", env["UPLOAD_LOCATION"])
	}
	if val, ok := env["EMPTY_VAR"]; !ok || val != "" {
		t.Errorf("expected EMPTY_VAR to be empty string, got %q (ok=%v)", val, ok)
	}
}

func TestInterpolateCompose(t *testing.T) {
	env := map[string]string{
		"IMAGE_TAG":  "v1.0.0",
		"APP_PORT":   "8080",
		"EMPTY_VAL":  "",
		"CUSTOM_MSG": "hello world",
	}

	tests := []struct {
		name     string
		input    string
		expected string
		hasErr   bool
	}{
		{
			name:     "braced variable present",
			input:    "image: myapp:${IMAGE_TAG}",
			expected: "image: myapp:v1.0.0",
		},
		{
			name:     "braced variable unset with default :-",
			input:    "image: ghcr.io/immich-app/immich-server:${IMMICH_VERSION:-release}",
			expected: "image: ghcr.io/immich-app/immich-server:release",
		},
		{
			name:     "braced variable empty with default :-",
			input:    "val: ${EMPTY_VAL:-fallback}",
			expected: "val: fallback",
		},
		{
			name:     "braced variable unset without default",
			input:    "port: ${UNSET_PORT}",
			expected: "port: ",
		},
		{
			name:     "escaped dollar sign $$",
			input:    "command: echo $$FOO",
			expected: "command: echo $FOO",
		},
		{
			name:     "simple variable $VAR",
			input:    "port: $APP_PORT",
			expected: "port: 8080",
		},
		{
			name:     "required variable present",
			input:    "msg: ${CUSTOM_MSG:?is required}",
			expected: "msg: hello world",
		},
		{
			name:   "required variable missing triggers error",
			input:  "secret: ${MISSING_SECRET:?missing secret}",
			hasErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := docker.InterpolateCompose(tc.input, env)
			if tc.hasErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}
