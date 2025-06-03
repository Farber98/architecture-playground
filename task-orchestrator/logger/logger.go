package logger

import (
	"log"
	"strings"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var currentLevel = INFO

// SetLevel configures the minimum log level
func SetLevel(level string) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		currentLevel = DEBUG
	case "INFO":
		currentLevel = INFO
	case "WARN":
		currentLevel = WARN
	case "ERROR":
		currentLevel = ERROR
	}
}

func Debugf(format string, args ...any) {
	if currentLevel <= DEBUG {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func Infof(format string, args ...any) {
	if currentLevel <= INFO {
		log.Printf("[INFO] "+format, args...)
	}
}

func Warnf(format string, args ...any) {
	if currentLevel <= WARN {
		log.Printf("[WARN] "+format, args...)
	}
}

func Errorf(format string, args ...any) {
	if currentLevel <= ERROR {
		log.Printf("[ERROR] "+format, args...)
	}
}

func Taskf(taskID, format string, args ...any) {
	if currentLevel <= INFO {
		log.Printf("[task_id=%s] "+format, append([]any{taskID}, args...)...)
	}
}
