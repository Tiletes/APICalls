package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	logDir  string
	logStem string
	logExt  string
	mu      sync.Mutex
)

// Init sets the log base path and creates parent directories.
// The actual log files are named <stem>-YYYY-MM-DD<ext> and rotate daily.
func Init(path string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".log"
	}
	logDir = dir
	logStem = stem
	logExt = ext
	return os.MkdirAll(dir, 0755)
}

// dailyLogPath returns the log file path for today's date.
func dailyLogPath() string {
	date := time.Now().Format("2006-01-02")
	return filepath.Join(logDir, logStem+"-"+date+logExt)
}

// Log writes a structured entry to today's log file.
// Format: YYYY-MM-DD:HH:mm:ss | user | module | description
func Log(user, module, description string) {
	mu.Lock()
	defer mu.Unlock()

	ts := time.Now().Format("2006-01-02:15:04:05")
	entry := fmt.Sprintf("%s | %-20s | %-20s | %s\n", ts, user, module, description)

	f, err := os.OpenFile(dailyLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger error: %v\n", err)
		return
	}
	defer f.Close()
	f.WriteString(entry)
}
