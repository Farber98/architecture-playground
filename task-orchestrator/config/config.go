package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	ServerPort      int           `json:"server_port"`
	TaskTimeout     time.Duration `json:"task_timeout"`
	LogLevel        string        `json:"log_level"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	Version         string        `json:"version"`

	// V2
	Async       bool   `json:"async"` // Enable V2 async processing
	RedisURL    string `json:"redis_url"`
	WorkerCount int    `json:"worker_count"` // Number of background workers
	QueueName   string `json:"queue_name"`   // Redis queue name
}

// LoadConfig loads configuration from environment variables with sensible defaults
func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServerPort:      getEnvInt("PORT", 8080),
		LogLevel:        getEnvString("LOG_LEVEL", "INFO"),
		TaskTimeout:     getEnvDuration("TASK_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		Version:         getEnvString("VERSION", "1.0.0"),
		Async:           getEnvBool("ASYNC_MODE", false),
		RedisURL:        getEnvString("REDIS_URL", "redis://localhost:6379"),
		WorkerCount:     getEnvInt("WORKER_COUNT", 3),
		QueueName:       getEnvString("QUEUE_NAME", "tasks"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// validate performs basic validation of the configuration
func (c *Config) validate() error {
	// Validate ServerPort
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return fmt.Errorf("invalid server port %d: must be between 1 and 65535", c.ServerPort)
	}

	// Validate and normalize LogLevel
	validLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true, "FATAL": true,
	}
	upperLevel := strings.ToUpper(strings.TrimSpace(c.LogLevel))
	if !validLevels[upperLevel] {
		return fmt.Errorf("invalid log level '%s': must be DEBUG, INFO, WARN, ERROR, or FATAL", c.LogLevel)
	}
	c.LogLevel = upperLevel

	// Validate TaskTimeout
	if c.TaskTimeout <= 0 {
		return fmt.Errorf("invalid task timeout %v: must be positive", c.TaskTimeout)
	}
	if c.TaskTimeout > 24*time.Hour {
		return fmt.Errorf("invalid task timeout %v: must not exceed 24 hours", c.TaskTimeout)
	}

	// Validate ShutdownTimeout
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("invalid shutdown timeout %v: must be positive", c.ShutdownTimeout)
	}
	if c.ShutdownTimeout > 5*time.Minute {
		return fmt.Errorf("invalid shutdown timeout %v: must not exceed 5 minutes", c.ShutdownTimeout)
	}

	// Validate Version
	if strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("version cannot be empty")
	}
	c.Version = strings.TrimSpace(c.Version)

	if c.Async {
		if c.WorkerCount < 1 {
			return fmt.Errorf("worker count must be at least 1 when async mode is enabled")
		}
		if strings.TrimSpace(c.RedisURL) == "" {
			return fmt.Errorf("redis URL cannot be empty when async mode is enabled")
		}
		if strings.TrimSpace(c.QueueName) == "" {
			return fmt.Errorf("queue name cannot be empty when async mode is enabled")
		}
	}

	return nil
}
