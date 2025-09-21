package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		logLevel    string
		expectError bool
	}{
		{
			name:        "valid DEBUG level",
			logLevel:    "DEBUG",
			expectError: false,
		},
		{
			name:        "valid INFO level",
			logLevel:    "INFO",
			expectError: false,
		},
		{
			name:        "valid WARN level",
			logLevel:    "WARN",
			expectError: false,
		},
		{
			name:        "valid ERROR level",
			logLevel:    "ERROR",
			expectError: false,
		},
		{
			name:        "invalid log level",
			logLevel:    "INVALID",
			expectError: true,
		},
		{
			name:        "empty log level",
			logLevel:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.logLevel)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, logger)
				assert.Contains(t, err.Error(), "invalid log level")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.Logger)
			}
		})
	}
}

func TestWithRequestFields(t *testing.T) {
	t.Parallel()
	// Create a logger for testing
	logger, err := New("INFO")
	require.NoError(t, err)

	// Create a test request
	req, err := http.NewRequest("GET", "http://example.com/api/v1/query?q=up", nil)
	require.NoError(t, err)
	req.RemoteAddr = "192.168.1.1:12345"

	// Test WithRequestFields without additional fields
	t.Run("with basic request fields", func(t *testing.T) {
		requestLogger := logger.WithRequestFields(req)

		assert.NotNil(t, requestLogger)
		assert.NotNil(t, requestLogger.Logger)
		assert.NotEqual(t, logger, requestLogger) // Should be a new instance
	})

	// Test WithRequestFields with additional fields
	t.Run("with additional fields", func(t *testing.T) {
		requestLogger := logger.WithRequestFields(req, "custom_field", "custom_value", "another_field", 42)

		assert.NotNil(t, requestLogger)
		assert.NotNil(t, requestLogger.Logger)
	})
}

func TestLogLevelMap(t *testing.T) {
	t.Parallel()
	expectedLevels := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}

	for levelStr := range expectedLevels {
		t.Run(levelStr, func(t *testing.T) {
			logger, err := New(levelStr)
			require.NoError(t, err)

			// We can't directly access the level from the logger, but we can test
			// that the logger was created successfully
			assert.NotNil(t, logger)
		})
	}
}

// TestLoggerOutput tests that the logger actually outputs JSON logs
func TestLoggerOutput(t *testing.T) {
	t.Parallel()
	// Capture log output
	var buf bytes.Buffer

	// Create a logger with a custom handler that writes to our buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := &Logger{slog.New(handler)}

	// Log a test message
	testLogger.Info("test message", "key1", "value1", "key2", 42)

	// Parse the JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify the log entry
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "value1", logEntry["key1"])
	assert.Equal(t, float64(42), logEntry["key2"]) // JSON numbers are floats
	assert.NotEmpty(t, logEntry["time"])
}

// TestWithRequestFieldsContent tests that WithRequestFields includes the expected fields
func TestWithRequestFieldsContent(t *testing.T) {
	t.Parallel()
	// Capture log output
	var buf bytes.Buffer

	// Create a logger with a custom handler that writes to our buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := &Logger{slog.New(handler)}

	// Create a test request
	req, err := http.NewRequest("POST", "http://example.com/api/v1/query_range?step=1m", strings.NewReader("query=up"))
	require.NoError(t, err)
	req.RemoteAddr = "192.168.1.1:12345"

	// Create request logger and log a message
	requestLogger := testLogger.WithRequestFields(req)
	requestLogger.Info("test message")

	// Parse the JSON output
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify the request fields are included
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "POST", logEntry["method"])
	assert.Equal(t, "http://example.com/api/v1/query_range?step=1m", logEntry["url"])
	assert.Equal(t, "192.168.1.1:12345", logEntry["remote_addr"])
	assert.NotEmpty(t, logEntry["request_id"])
	assert.NotEmpty(t, logEntry["time"])

	// Verify request_id is a valid UUID format (basic check)
	requestID, ok := logEntry["request_id"].(string)
	assert.True(t, ok)
	assert.Len(t, requestID, 36) // UUID is 36 characters with hyphens
	assert.Contains(t, requestID, "-")
}

func TestLogLevelMapCoverage(t *testing.T) {
	t.Parallel()
	// Test that logLevelMap contains all expected levels
	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}

	for _, level := range expectedLevels {
		t.Run(level, func(t *testing.T) {
			t.Parallel()
			_, exists := LogLevelMap[level]
			assert.True(t, exists, "logLevelMap should contain level %s", level)
		})
	}
}

// Benchmark tests
func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		logger, err := New("INFO")
		if err != nil {
			b.Fatal(err)
		}
		_ = logger
	}
}

func BenchmarkWithRequestFields(b *testing.B) {
	logger, err := New("INFO")
	if err != nil {
		b.Fatal(err)
	}

	req, err := http.NewRequest("GET", "http://example.com/test", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		requestLogger := logger.WithRequestFields(req)
		_ = requestLogger
	}
}
