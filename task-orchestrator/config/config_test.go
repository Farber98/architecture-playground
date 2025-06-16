package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Clearenv()

	cfg, err := LoadConfig()

	assert.NilError(t, err)
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

	cfg, err := LoadConfig()

	assert.NilError(t, err)
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

	cfg, err := LoadConfig()

	// Should fall back to defaults and validate successfully
	assert.NilError(t, err)
	assert.Equal(t, 8080, cfg.ServerPort)
	assert.Equal(t, 30*time.Second, cfg.TaskTimeout)
}

// Validation Tests

func TestLoadConfig_ValidationSuccess(t *testing.T) {
	os.Clearenv()
	os.Setenv("PORT", "8080")
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("TASK_TIMEOUT", "30s")
	os.Setenv("SHUTDOWN_TIMEOUT", "15s")
	os.Setenv("VERSION", "1.0.0")

	defer os.Clearenv()

	cfg, err := LoadConfig()

	assert.NilError(t, err)
	assert.Assert(t, cfg != nil)
}

func TestLoadConfig_InvalidServerPort(t *testing.T) {
	tests := []struct {
		name string
		port string
	}{
		{"port too low", "0"},
		{"port too high", "65536"},
		{"negative port", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("PORT", tt.port)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.Assert(t, cfg == nil)
			assert.Assert(t, err != nil)
			assert.Assert(t, strings.Contains(err.Error(), "invalid server port"))
		})
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"invalid level", "INVALID"},
		{"numeric level", "123"},
		{"random string", "random"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("LOG_LEVEL", tt.logLevel)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.Assert(t, cfg == nil)
			assert.Assert(t, err != nil)
			assert.Assert(t, strings.Contains(err.Error(), "invalid log level"))
		})
	}
}

func TestLoadConfig_LogLevelNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase debug", "debug", "DEBUG"},
		{"lowercase info", "info", "INFO"},
		{"mixed case warn", "WaRn", "WARN"},
		{"with spaces", " ERROR ", "ERROR"},
		{"lowercase fatal", "fatal", "FATAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("LOG_LEVEL", tt.input)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, cfg.LogLevel)
		})
	}
}

func TestLoadConfig_InvalidTaskTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		errorMsg string
	}{
		{"negative timeout", "-5s", "must be positive"},
		{"zero timeout", "0", "must be positive"},
		{"too large timeout", "25h", "must not exceed 24 hours"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("TASK_TIMEOUT", tt.timeout)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.Assert(t, cfg == nil)
			assert.Assert(t, err != nil)
			assert.Assert(t, strings.Contains(err.Error(), "invalid task timeout"))
			assert.Assert(t, strings.Contains(err.Error(), tt.errorMsg))
		})
	}
}

func TestLoadConfig_ValidTaskTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		expected time.Duration
	}{
		{"1 second", "1s", 1 * time.Second},
		{"30 seconds", "30s", 30 * time.Second},
		{"5 minutes", "5m", 5 * time.Minute},
		{"1 hour", "1h", 1 * time.Hour},
		{"24 hours", "24h", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("TASK_TIMEOUT", tt.timeout)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, cfg.TaskTimeout)
		})
	}
}

func TestLoadConfig_InvalidShutdownTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		errorMsg string
	}{
		{"negative timeout", "-5s", "must be positive"},
		{"zero timeout", "0", "must be positive"},
		{"too large timeout", "6m", "must not exceed 5 minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("SHUTDOWN_TIMEOUT", tt.timeout)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.Assert(t, cfg == nil)
			assert.Assert(t, err != nil)
			assert.Assert(t, strings.Contains(err.Error(), "invalid shutdown timeout"))
			assert.Assert(t, strings.Contains(err.Error(), tt.errorMsg))
		})
	}
}

func TestLoadConfig_ValidShutdownTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		expected time.Duration
	}{
		{"1 second", "1s", 1 * time.Second},
		{"15 seconds", "15s", 15 * time.Second},
		{"1 minute", "1m", 1 * time.Minute},
		{"5 minutes", "5m", 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("SHUTDOWN_TIMEOUT", tt.timeout)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, cfg.ShutdownTimeout)
		})
	}
}

func TestLoadConfig_InvalidVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"only spaces", "   "},
		{"only tabs", "\t\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("VERSION", tt.version)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.Assert(t, cfg == nil)
			assert.Assert(t, err != nil)
			assert.Assert(t, strings.Contains(err.Error(), "version cannot be empty"))
		})
	}
}

func TestLoadConfig_VersionNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with leading spaces", "  1.0.0", "1.0.0"},
		{"with trailing spaces", "1.0.0  ", "1.0.0"},
		{"with both spaces", "  2.0.0-beta  ", "2.0.0-beta"},
		{"semantic version", "1.2.3", "1.2.3"},
		{"with build info", "1.0.0+build.123", "1.0.0+build.123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("VERSION", tt.input)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, cfg.Version)
		})
	}
}

func TestLoadConfig_ValidPortEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		expected int
	}{
		{"minimum port", "1", 1},
		{"maximum port", "65535", 65535},
		{"common development port", "3000", 3000},
		{"common production port", "8080", 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("PORT", tt.port)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.NilError(t, err)
			assert.Equal(t, tt.expected, cfg.ServerPort)
		})
	}
}

func TestLoadConfig_AllValidLogLevels(t *testing.T) {
	validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	for _, level := range validLevels {
		t.Run("level_"+level, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("LOG_LEVEL", level)
			defer os.Clearenv()

			cfg, err := LoadConfig()

			assert.NilError(t, err)
			assert.Equal(t, level, cfg.LogLevel)
		})
	}
}

func TestLoadConfig_EnvParsingFallbacks(t *testing.T) {
	// Test that invalid env values fall back to defaults and still validate
	os.Clearenv()
	os.Setenv("PORT", "invalid-port")         // Falls back to 8080
	os.Setenv("TASK_TIMEOUT", "invalid-time") // Falls back to 30s
	os.Setenv("SHUTDOWN_TIMEOUT", "invalid")  // Falls back to 15s
	os.Setenv("LOG_LEVEL", "INFO")            // Valid
	os.Setenv("VERSION", "1.0.0")             // Valid

	defer os.Clearenv()

	cfg, err := LoadConfig()

	assert.NilError(t, err)
	assert.Equal(t, 8080, cfg.ServerPort)                // Default value
	assert.Equal(t, 30*time.Second, cfg.TaskTimeout)     // Default value
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout) // Default value
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "1.0.0", cfg.Version)
}
