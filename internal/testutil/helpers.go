package testutil

import (
	"io"
	"net/http"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

// CreateTestLogger creates a logger for testing purposes
func CreateTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	logger, err := logger.New("ERROR") // Use ERROR level to reduce noise in tests
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}
	return logger
}

// CreateHTTPRequest creates a test HTTP request
func CreateHTTPRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("failed to create test request: %v", err)
	}
	return req
}
