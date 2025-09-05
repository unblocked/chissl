package chserver

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// LogManager handles log storage and retrieval
type LogManager struct {
	mu            sync.RWMutex
	logFile       *os.File
	logDir        string
	maxLogSize    int64
	maxLogFiles   int
	recentLogs    []LogEntry
	maxRecentLogs int
}

// NewLogManager creates a new log manager
func NewLogManager(logDir string) (*LogManager, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	lm := &LogManager{
		logDir:        logDir,
		maxLogSize:    10 * 1024 * 1024, // 10MB per log file
		maxLogFiles:   10,               // Keep 10 log files
		maxRecentLogs: 1000,             // Keep 1000 recent logs in memory
		recentLogs:    make([]LogEntry, 0, 1000),
	}

	if err := lm.openLogFile(); err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return lm, nil
}

// openLogFile opens the current log file
func (lm *LogManager) openLogFile() error {
	logPath := filepath.Join(lm.logDir, "chissl.log")

	var err error
	lm.logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	return nil
}

// WriteLog writes a log entry to file and memory
func (lm *LogManager) WriteLog(level, message, source string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	// Write to file
	if lm.logFile != nil {
		logLine := fmt.Sprintf("[%s] %s: %s\n",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			strings.ToUpper(entry.Level),
			entry.Message)

		if source != "" {
			logLine = fmt.Sprintf("[%s] %s [%s]: %s\n",
				entry.Timestamp.Format("2006-01-02 15:04:05"),
				strings.ToUpper(entry.Level),
				source,
				entry.Message)
		}

		lm.logFile.WriteString(logLine)
		lm.logFile.Sync()

		// Check if we need to rotate the log file
		if stat, err := lm.logFile.Stat(); err == nil && stat.Size() > lm.maxLogSize {
			lm.rotateLogFile()
		}
	}

	// Add to recent logs in memory
	lm.recentLogs = append(lm.recentLogs, entry)
	if len(lm.recentLogs) > lm.maxRecentLogs {
		// Remove oldest entries
		copy(lm.recentLogs, lm.recentLogs[len(lm.recentLogs)-lm.maxRecentLogs:])
		lm.recentLogs = lm.recentLogs[:lm.maxRecentLogs]
	}
}

// rotateLogFile rotates the current log file
func (lm *LogManager) rotateLogFile() {
	if lm.logFile != nil {
		lm.logFile.Close()
	}

	// Rename current log file with timestamp
	currentPath := filepath.Join(lm.logDir, "chissl.log")
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := filepath.Join(lm.logDir, fmt.Sprintf("chissl-%s.log", timestamp))

	os.Rename(currentPath, rotatedPath)

	// Clean up old log files
	lm.cleanupOldLogs()

	// Open new log file
	lm.openLogFile()
}

// cleanupOldLogs removes old log files beyond the retention limit
func (lm *LogManager) cleanupOldLogs() {
	files, err := filepath.Glob(filepath.Join(lm.logDir, "chissl-*.log"))
	if err != nil {
		return
	}

	if len(files) <= lm.maxLogFiles {
		return
	}

	// Sort files by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		statI, errI := os.Stat(files[i])
		statJ, errJ := os.Stat(files[j])
		if errI != nil || errJ != nil {
			return false
		}
		return statI.ModTime().Before(statJ.ModTime())
	})

	// Remove oldest files
	for i := 0; i < len(files)-lm.maxLogFiles; i++ {
		os.Remove(files[i])
	}
}

// GetRecentLogs returns recent log entries
func (lm *LogManager) GetRecentLogs(limit int) []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if limit <= 0 || limit > len(lm.recentLogs) {
		limit = len(lm.recentLogs)
	}

	// Return most recent logs (reverse order)
	result := make([]LogEntry, limit)
	start := len(lm.recentLogs) - limit
	for i := 0; i < limit; i++ {
		result[limit-1-i] = lm.recentLogs[start+i]
	}

	return result
}

// GetLogFiles returns available log files
func (lm *LogManager) GetLogFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(lm.logDir, "chissl*.log"))
	if err != nil {
		return nil, err
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		statI, errI := os.Stat(files[i])
		statJ, errJ := os.Stat(files[j])
		if errI != nil || errJ != nil {
			return false
		}
		return statI.ModTime().After(statJ.ModTime())
	})

	// Return just the filenames
	result := make([]string, len(files))
	for i, file := range files {
		result[i] = filepath.Base(file)
	}

	return result, nil
}

// ReadLogFile reads a specific log file
func (lm *LogManager) ReadLogFile(filename string, lines int) ([]string, error) {
	// Sanitize filename to prevent directory traversal
	filename = filepath.Base(filename)
	if !strings.HasPrefix(filename, "chissl") || !strings.HasSuffix(filename, ".log") {
		return nil, fmt.Errorf("invalid log file name")
	}

	filePath := filepath.Join(lm.logDir, filename)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var logLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last N lines if specified
	if lines > 0 && lines < len(logLines) {
		return logLines[len(logLines)-lines:], nil
	}

	return logLines, nil
}

// Close closes the log manager
func (lm *LogManager) Close() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.logFile != nil {
		return lm.logFile.Close()
	}
	return nil
}

// LogWriter implements io.Writer interface for integration with standard loggers
type LogWriter struct {
	logManager *LogManager
	level      string
	source     string
}

// NewLogWriter creates a new log writer
func NewLogWriter(logManager *LogManager, level, source string) *LogWriter {
	return &LogWriter{
		logManager: logManager,
		level:      level,
		source:     source,
	}
}

// Write implements io.Writer
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		lw.logManager.WriteLog(lw.level, message, lw.source)
	}
	return len(p), nil
}
