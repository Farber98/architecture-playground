package logger

import (
	"encoding/json"
	"io"
	"log"
	"maps"
	"os"
	"strings"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// Logger represents a configurable logger instance
type Logger struct {
	level  Level
	logger *log.Logger
}

// LogEntry represents a structured log entry
type logEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// New creates a new logger with simple parameters
func New(level string, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	return &Logger{
		level:  parseLevel(level),
		logger: log.New(output, "", 0),
	}
}

// parseLevel converts string to Level (internal function)
func parseLevel(level string) Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// logEntry writes a structured log entry
func (l *Logger) writeLogEntry(level Level, message string, fields map[string]any) {
	if l.level > level {
		return
	}

	entry := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     getLevelName(level),
		Message:   message,
		Fields:    fields,
	}

	if data, err := json.Marshal(entry); err == nil {
		l.logger.Println(string(data))
	} else {
		// Fallback to simple format if JSON fails
		l.logger.Printf("[%s] %s", entry.Level, message)
	}
}

func getLevelName(level Level) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Core logging methods - always structured
func (l *Logger) Debug(message string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.writeLogEntry(DEBUG, message, f)
}

func (l *Logger) Info(message string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.writeLogEntry(INFO, message, f)
}

func (l *Logger) Warn(message string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.writeLogEntry(WARN, message, f)
}

func (l *Logger) Error(message string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 {
		f = fields[0]
	}
	l.writeLogEntry(ERROR, message, f)
}

// Specialized logging methods
func (l *Logger) Task(taskID, message string, fields ...map[string]any) {
	allFields := map[string]any{
		"task_id": taskID,
		"type":    "task",
	}

	if len(fields) > 0 && fields[0] != nil {
		maps.Copy(allFields, fields[0])
	}

	l.writeLogEntry(INFO, message, allFields)
}

func (l *Logger) HTTP(method, path string, statusCode int, duration time.Duration, fields ...map[string]any) {
	allFields := map[string]any{
		"http_method": method,
		"http_path":   path,
		"http_status": statusCode,
		"duration_ns": duration.Nanoseconds(),
		"type":        "http_request",
	}

	if len(fields) > 0 && fields[0] != nil {
		maps.Copy(allFields, fields[0])
	}

	l.writeLogEntry(INFO, "HTTP request completed", allFields)
}
