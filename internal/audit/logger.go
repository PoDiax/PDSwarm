package audit

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level represents the active audit log filtering level.
type Level string

const (
	LevelAuth         Level = "auth"          // Log all auth events (logins, failures, password changes, tokens)
	LevelFailuresOnly Level = "failures_only" // Log only failed logins and unauthorized attempts (strictest)
	LevelAll          Level = "all"           // Log all auth events and warnings
	LevelOff          Level = "off"           // File logging disabled
)

// Logger provides thread-safe security audit logging to a dedicated file
// formatted specifically for security jail engines like Fail2ban and CrowdSec.
type Logger struct {
	mu       sync.Mutex
	path     string
	level    Level
	file     *os.File
	writer   io.Writer
	port     int
}

// Global default instance
var (
	defaultLogger *Logger
	defaultOnce   sync.Once
)

// SetDefault sets the package-level default audit logger.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Default returns the active default audit logger, creating an in-memory/stderr one if uninitialized.
func Default() *Logger {
	defaultOnce.Do(func() {
		if defaultLogger == nil {
			defaultLogger = &Logger{
				level:  LevelAuth,
				writer: os.Stderr,
			}
		}
	})
	return defaultLogger
}

// New initializes an audit logger for the given file path and level.
// It ensures the parent directory exists and opens the file in append mode.
func New(path, levelStr string, port int) (*Logger, error) {
	level := Level(strings.ToLower(strings.TrimSpace(levelStr)))
	if level == "" {
		level = LevelAuth
	}

	l := &Logger{
		path:  path,
		level: level,
		port:  port,
	}

	if level == LevelOff || path == "" {
		return l, nil
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Printf("[WARN] Failed to create audit log directory at %s: %v", dir, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		log.Printf("[WARN] Could not open audit log file at %s: %v (falling back to stdout)", path, err)
		l.writer = os.Stderr
		return l, err
	}

	l.file = f
	l.writer = io.MultiWriter(f, os.Stdout)
	return l, nil
}

// Path returns the path to the configured audit log file.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Level returns the active logging level.
func (l *Logger) Level() Level {
	if l == nil {
		return LevelOff
	}
	return l.level
}

// Close closes the underlying log file handle.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// writeLine formats and writes a single line to the audit log.
func (l *Logger) writeLine(tag, status, message string) {
	if l == nil || l.level == LevelOff {
		return
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")
	line := fmt.Sprintf("%s [%s] [%s] %s\n", timestamp, tag, status, message)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		_, _ = l.writer.Write([]byte(line))
	} else {
		log.Print(line)
	}
}

// LogLoginSuccess logs a successful user authentication event.
func (l *Logger) LogLoginSuccess(ip, username, userAgent string) {
	if l == nil || l.level == LevelOff || l.level == LevelFailuresOnly {
		return
	}
	msg := fmt.Sprintf("Login successful for user=%q from ip=%s", username, ip)
	if userAgent != "" {
		msg += fmt.Sprintf(" agent=%q", userAgent)
	}
	l.writeLine("AUTH", "SUCCESS", msg)
}

// LogLoginFailure logs a failed login attempt (crucial for Fail2ban/CrowdSec).
func (l *Logger) LogLoginFailure(ip, username, reason, userAgent string) {
	if l == nil || l.level == LevelOff {
		return
	}
	msg := fmt.Sprintf("Login failed for user=%q from ip=%s: %s", username, ip, reason)
	if userAgent != "" {
		msg += fmt.Sprintf(" agent=%q", userAgent)
	}
	l.writeLine("AUTH", "FAIL", msg)
}

// LogUnauthorized logs an unauthorized API access or invalid/expired token.
func (l *Logger) LogUnauthorized(ip, path, reason string) {
	if l == nil || l.level == LevelOff {
		return
	}
	msg := fmt.Sprintf("Unauthorized access from ip=%s path=%s: %s", ip, path, reason)
	l.writeLine("AUTH", "FAIL", msg)
}

// LogPasswordChange logs a password update event.
func (l *Logger) LogPasswordChange(ip, username string) {
	if l == nil || l.level == LevelOff || l.level == LevelFailuresOnly {
		return
	}
	msg := fmt.Sprintf("Password updated for user=%q from ip=%s", username, ip)
	l.writeLine("AUTH", "INFO", msg)
}

// LogSetup logs the initial administrative account creation.
func (l *Logger) LogSetup(ip, username string) {
	if l == nil || l.level == LevelOff || l.level == LevelFailuresOnly {
		return
	}
	msg := fmt.Sprintf("Initial setup completed, admin user=%q created from ip=%s", username, ip)
	l.writeLine("AUTH", "SUCCESS", msg)
}

// LogTokenAction logs API access token creation or revocation.
func (l *Logger) LogTokenAction(ip, username, action, tokenName string) {
	if l == nil || l.level == LevelOff || l.level == LevelFailuresOnly {
		return
	}
	msg := fmt.Sprintf("API token %s %q by user=%q from ip=%s", action, tokenName, username, ip)
	l.writeLine("AUTH", "INFO", msg)
}

// ClientIP extracts the real remote IP address of the client,
// properly resolving headers when behind reverse proxies (Nginx, Traefik, Caddy, Cloudflare).
func ClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}

	// 1. Cloudflare connecting IP
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}

	// 2. X-Forwarded-For (take the first client IP in comma-separated chain)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 3. X-Real-IP
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}

	// 4. RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}

	return "unknown"
}

// ReadRecent reads the last maxLines from the audit log file.
func (l *Logger) ReadRecent(maxLines int) ([]string, error) {
	if l == nil || l.path == "" {
		return []string{}, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) != "" {
			lines = append(lines, text)
		}
	}

	if err := scanner.Err(); err != nil {
		return lines, err
	}

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return lines, nil
}

// Fail2banFilterConfig returns a copy-pasteable Fail2ban filter definition for PDSwarm.
func (l *Logger) Fail2banFilterConfig() string {
	return `# /etc/fail2ban/filter.d/pdswarm.conf
# Fail2ban filter definition for PDSwarm Web Management
# Detects failed login attempts and unauthorized token probes

[Definition]
failregex = ^.*\[AUTH\] \[FAIL\] (?:Login failed for user=".*"|Unauthorized access) from ip=<HOST>
ignoreregex =
`
}

// Fail2banJailConfig returns a copy-pasteable Fail2ban jail definition configured with this host's settings.
func (l *Logger) Fail2banJailConfig() string {
	port := l.port
	if port <= 0 {
		port = 9000
	}
	logPath := l.path
	if logPath == "" {
		logPath = "/var/lib/pdswarm/auth.log"
	}

	return fmt.Sprintf(`# /etc/fail2ban/jail.d/pdswarm.conf
# Fail2ban jail configuration for PDSwarm

[pdswarm]
enabled  = true
port     = %d
filter   = pdswarm
logpath  = %s
maxretry = 5
findtime = 600
bantime  = 3600
action   = iptables-multiport[name=pdswarm, port="%d", protocol=tcp]
`, port, logPath, port)
}
