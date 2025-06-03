package config

import (
	"os"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any existing env vars
	os.Clearenv()

	cfg := LoadConfig()

	assert.Equal(t, 8080, cfg.ServerPort)
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, 30*time.Second, cfg.TaskTimeout)
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, ":8080", cfg.Address())
}

func TestLoadConfig_FromEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("LOG_LEVEL", "DEBUG")
	os.Setenv("TASK_TIMEOUT", "45s")
	os.Setenv("SHUTDOWN_TIMEOUT", "20s")
	os.Setenv("VERSION", "2.0.0-beta")

	defer func() {
		os.Clearenv()
	}()

	cfg := LoadConfig()

	assert.Equal(t, 9000, cfg.ServerPort)
	assert.Equal(t, "DEBUG", cfg.LogLevel)
	assert.Equal(t, 45*time.Second, cfg.TaskTimeout)
	assert.Equal(t, 20*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, "2.0.0-beta", cfg.Version)
	assert.Equal(t, ":9000", cfg.Address())
}

func TestLoadConfig_InvalidValues(t *testing.T) {
	// Set invalid environment variables
	os.Setenv("PORT", "not-a-number")
	os.Setenv("TASK_TIMEOUT", "not-a-duration")

	defer func() {
		os.Clearenv()
	}()

	cfg := LoadConfig()

	// Should fall back to defaults
	assert.Equal(t, 8080, cfg.ServerPort)
	assert.Equal(t, 30*time.Second, cfg.TaskTimeout)
}
