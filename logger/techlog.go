package logger

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ── Levels ────────────────────────────────────────────────────────────────────

const (
	LevelInfo  = "INFO "
	LevelWarn  = "WARN "
	LevelError = "ERROR"
)

// ── Error classification ───────────────────────────────────────────────────────

// ErrType categorises a network/HTTP error for structured logging.
type ErrType string

const (
	ErrTypeTimeout    ErrType = "TIMEOUT"
	ErrTypeConnection ErrType = "CONNECTION_ERROR"
	ErrTypeDNS        ErrType = "DNS_ERROR"
	ErrTypeHTTPS      ErrType = "TLS_ERROR"
	ErrTypeNetwork    ErrType = "NETWORK_ERROR"
	ErrTypeRequest    ErrType = "REQUEST_ERROR"
	ErrTypeHTTP4xx    ErrType = "HTTP_CLIENT_ERROR"
	ErrTypeHTTP5xx    ErrType = "HTTP_SERVER_ERROR"
	ErrTypeDatabase   ErrType = "DB_ERROR"
)

// ClassifyNetErr maps a network-level error to an ErrType.
func ClassifyNetErr(err error) ErrType {
	if err == nil {
		return ""
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrTypeTimeout
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		inner := urlErr.Err.Error()
		if strings.Contains(inner, "certificate") || strings.Contains(inner, "tls") || strings.Contains(inner, "x509") {
			return ErrTypeHTTPS
		}
		if strings.Contains(inner, "no such host") || strings.Contains(inner, "lookup") {
			return ErrTypeDNS
		}
		if strings.Contains(inner, "connection refused") || strings.Contains(inner, "connect:") {
			return ErrTypeConnection
		}
		if strings.Contains(inner, "timeout") || strings.Contains(inner, "deadline") {
			return ErrTypeTimeout
		}
	}
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "connect:") {
		return ErrTypeConnection
	}
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup") {
		return ErrTypeDNS
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return ErrTypeTimeout
	}
	return ErrTypeNetwork
}

// ClassifyHTTPStatus returns an ErrType for HTTP 4xx/5xx status codes, or "".
func ClassifyHTTPStatus(code int) ErrType {
	switch {
	case code >= 500:
		return ErrTypeHTTP5xx
	case code >= 400:
		return ErrTypeHTTP4xx
	default:
		return ""
	}
}

// ── Writer ────────────────────────────────────────────────────────────────────

var (
	techDir  string
	techStem string
	techExt  string
	techMu   sync.Mutex
)

// TechInit sets the technical log base path and creates parent directories.
// The actual log files are named <stem>-YYYY-MM-DD<ext> and rotate daily.
func TechInit(path string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".log"
	}
	techDir = dir
	techStem = stem
	techExt = ext
	return os.MkdirAll(dir, 0755)
}

// dailyTechPath returns the technical log file path for today's date.
func dailyTechPath() string {
	date := time.Now().Format("2006-01-02")
	return filepath.Join(techDir, techStem+"-"+date+techExt)
}

// Tech writes a structured entry to today's technical log file.
//
// Format: YYYY-MM-DD HH:mm:ss.mmm | LEVEL | user                 | module               | message
func Tech(level, user, module, message string) {
	if techDir == "" {
		return
	}
	techMu.Lock()
	defer techMu.Unlock()

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	entry := fmt.Sprintf("%s | %s | %-20s | %-20s | %s\n",
		ts, level,
		truncate(user, 20),
		truncate(module, 20),
		message,
	)

	f, err := os.OpenFile(dailyTechPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "techlog error: %v\n", err)
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

// TechInfo is a convenience wrapper for INFO-level entries.
func TechInfo(user, module, message string) { Tech(LevelInfo, user, module, message) }

// TechWarn is a convenience wrapper for WARN-level entries.
func TechWarn(user, module, message string) { Tech(LevelWarn, user, module, message) }

// TechError is a convenience wrapper for ERROR-level entries.
func TechError(user, module, message string) { Tech(LevelError, user, module, message) }

// ── Helpers ───────────────────────────────────────────────────────────────────

// truncate clips a string to n runes, padding with spaces if shorter.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n])
	}
	return s
}

// FormatBytes returns a human-friendly byte count (B, KB, MB).
func FormatBytes(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
