package docker

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ParseEnvFile parses a .env file format into a key-value map.
// It supports comments (#), export statements, and quoted values.
func ParseEnvFile(content string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove leading "export "
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip surrounding quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		if key != "" {
			result[key] = val
		}
	}

	return result
}

var (
	// bracedVarRegex matches ${VAR}, ${VAR:-default}, ${VAR-default}, ${VAR:?error}, ${VAR?error}
	bracedVarRegex = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?:(:?[-?])([^}]*))?\}`)
	// simpleVarRegex matches $VAR (when not preceded or followed by another $)
	simpleVarRegex = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)
)

const escapedDollarToken = "\x00_PDSWARM_ESCAPED_DOLLAR_\x00"

// InterpolateCompose replaces variable placeholders in Compose YAML using the provided environment map
// and host environment according to Docker Compose variable substitution specification.
func InterpolateCompose(content string, env map[string]string) (string, error) {
	if content == "" {
		return "", nil
	}

	// 1. Protect escaped $$ by replacing with a temporary token
	replaced := strings.ReplaceAll(content, "$$", escapedDollarToken)

	lookup := func(name string) (string, bool) {
		if env != nil {
			if v, ok := env[name]; ok {
				return v, true
			}
		}
		return os.LookupEnv(name)
	}

	var interpolationErr error

	// 2. Handle braced variables: ${VAR...}
	replaced = bracedVarRegex.ReplaceAllStringFunc(replaced, func(match string) string {
		if interpolationErr != nil {
			return match
		}

		sub := bracedVarRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}

		varName := sub[1]
		modifier := ""
		defaultOrErr := ""
		if len(sub) >= 3 {
			modifier = sub[2]
		}
		if len(sub) >= 4 {
			defaultOrErr = sub[3]
		}

		val, exists := lookup(varName)

		switch modifier {
		case ":-":
			// Default if unset or empty
			if !exists || val == "" {
				return defaultOrErr
			}
			return val
		case "-":
			// Default only if unset
			if !exists {
				return defaultOrErr
			}
			return val
		case ":?":
			// Error if unset or empty
			if !exists || val == "" {
				msg := defaultOrErr
				if msg == "" {
					msg = fmt.Sprintf("required variable %s is not set or empty", varName)
				}
				interpolationErr = fmt.Errorf("variable %s: %s", varName, msg)
				return match
			}
			return val
		case "?":
			// Error only if unset
			if !exists {
				msg := defaultOrErr
				if msg == "" {
					msg = fmt.Sprintf("required variable %s is not set", varName)
				}
				interpolationErr = fmt.Errorf("variable %s: %s", varName, msg)
				return match
			}
			return val
		default:
			// No modifier: ${VAR}
			if !exists {
				return ""
			}
			return val
		}
	})

	if interpolationErr != nil {
		return "", interpolationErr
	}

	// 3. Handle simple variables: $VAR
	replaced = simpleVarRegex.ReplaceAllStringFunc(replaced, func(match string) string {
		varName := strings.TrimPrefix(match, "$")
		if val, exists := lookup(varName); exists {
			return val
		}
		return ""
	})

	// 4. Restore escaped $$ to single $
	replaced = strings.ReplaceAll(replaced, escapedDollarToken, "$")

	return replaced, nil
}
