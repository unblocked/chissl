package chserver

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// LogsResponse represents the response for logs API
type LogsResponse struct {
	Logs       []LogEntry `json:"logs"`
	TotalCount int        `json:"total_count"`
	HasMore    bool       `json:"has_more"`
}

// LogFilesResponse represents the response for log files API
type LogFilesResponse struct {
	Files []LogFileInfo `json:"files"`
}

// LogFileInfo represents information about a log file
type LogFileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modified_time"`
}

// LogFileContentResponse represents the response for log file content
type LogFileContentResponse struct {
	Filename string   `json:"filename"`
	Lines    []string `json:"lines"`
	Total    int      `json:"total"`
}

// handleGetLogs returns recent log entries (admin only)
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	if s.logManager == nil {
		http.Error(w, "Logging not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 1000 { // max limit
				limit = 1000
			}
		}
	}

	// Get recent logs
	logs := s.logManager.GetRecentLogs(limit)

	response := LogsResponse{
		Logs:       logs,
		TotalCount: len(logs),
		HasMore:    len(logs) == limit, // Simplified - in a real system you'd track total available
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetLogFiles returns available log files (admin only)
func (s *Server) handleGetLogFiles(w http.ResponseWriter, r *http.Request) {
	if s.logManager == nil {
		http.Error(w, "Logging not configured", http.StatusServiceUnavailable)
		return
	}

	files, err := s.logManager.GetLogFiles()
	if err != nil {
		s.Debugf("Failed to get log files: %v", err)
		http.Error(w, "Failed to get log files", http.StatusInternalServerError)
		return
	}

	// Get file info for each file
	var fileInfos []LogFileInfo
	for _, filename := range files {
		// Get file stats
		filePath := s.logManager.logDir + "/" + filename
		if stat, err := os.Stat(filePath); err == nil {
			fileInfo := LogFileInfo{
				Name:         filename,
				Size:         stat.Size(),
				ModifiedTime: stat.ModTime().Format("2006-01-02 15:04:05"),
			}
			fileInfos = append(fileInfos, fileInfo)
		}
	}

	response := LogFilesResponse{
		Files: fileInfos,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetLogFileContent returns content of a specific log file (admin only)
func (s *Server) handleGetLogFileContent(w http.ResponseWriter, r *http.Request) {
	if s.logManager == nil {
		http.Error(w, "Logging not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract filename from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	filename := parts[3]

	// Parse query parameters
	linesStr := r.URL.Query().Get("lines")
	lines := 0 // 0 means all lines
	if linesStr != "" {
		if parsedLines, err := strconv.Atoi(linesStr); err == nil && parsedLines > 0 {
			lines = parsedLines
		}
	}

	// Read log file
	logLines, err := s.logManager.ReadLogFile(filename, lines)
	if err != nil {
		s.Debugf("Failed to read log file %s: %v", filename, err)
		http.Error(w, "Failed to read log file", http.StatusInternalServerError)
		return
	}

	response := LogFileContentResponse{
		Filename: filename,
		Lines:    logLines,
		Total:    len(logLines),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleClearLogs clears recent logs from memory (admin only)
func (s *Server) handleClearLogs(w http.ResponseWriter, r *http.Request) {
	if s.logManager == nil {
		http.Error(w, "Logging not configured", http.StatusServiceUnavailable)
		return
	}

	// Clear recent logs from memory (files remain)
	s.logManager.mu.Lock()
	s.logManager.recentLogs = s.logManager.recentLogs[:0]
	s.logManager.mu.Unlock()

	s.logManager.WriteLog("info", "Recent logs cleared by admin", "api")

	response := map[string]string{"status": "success", "message": "Recent logs cleared"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDownloadLogFile allows downloading a log file (admin only)
func (s *Server) handleDownloadLogFile(w http.ResponseWriter, r *http.Request) {
	if s.logManager == nil {
		http.Error(w, "Logging not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract filename from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	filename := parts[4]

	// Read entire log file
	logLines, err := s.logManager.ReadLogFile(filename, 0)
	if err != nil {
		s.Debugf("Failed to read log file %s: %v", filename, err)
		http.Error(w, "Failed to read log file", http.StatusNotFound)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// Write log content
	for _, line := range logLines {
		w.Write([]byte(line + "\n"))
	}
}
