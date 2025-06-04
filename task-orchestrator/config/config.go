package config

import (
	"fmt"
	"os"
	"strconv"
	"task-orchestrator/logger"
	"time"
)

// Config holds all application configuration
type Config struct {
	ServerPort      int           `json:"server_port"`
	TaskTimeout     time.Duration `json:"task_timeout"`
	LogLevel        string        `json:"log_level"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	Version         string        `json:"version"`

	Logger *logger.Logger `json:"-"`
}

// LoadConfig loads configuration from environment variables with sensible defaults
func LoadConfig() *Config {
	return &Config{
		ServerPort:      getEnvInt("PORT", 8080),
		LogLevel:        getEnvString("LOG_LEVEL", "INFO"),
		TaskTimeout:     getEnvDuration("TASK_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		Version:         getEnvString("VERSION", "1.0.0"),
	}
}

// Address returns the server address in host:port format
func (c *Config) Address() string {
	return fmt.Sprintf(":%d", c.ServerPort)
}

// Helper functions for environment variable parsing
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
