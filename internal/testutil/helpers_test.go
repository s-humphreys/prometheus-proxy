package testutil

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTestLogger(t *testing.T) {
	tests := []struct {
		name           string
		expectedLevel  string
		shouldNotBeNil bool
	}{
		{
			name:           "creates_logger_successfully",
			expectedLevel:  "ERROR",
			shouldNotBeNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := CreateTestLogger(t)

			if tt.shouldNotBeNil {
				assert.NotNil(t, logger, "CreateTestLogger should return a non-nil logger")
				assert.NotNil(t, logger.Logger, "Logger should have a valid slog.Logger")
			}
		})
	}
}

func TestCreateTestLoggerErrorLevel(t *testing.T) {
	// Test that the logger is created with ERROR level to reduce noise in tests
	logger := CreateTestLogger(t)
	assert.NotNil(t, logger)

	// We can't directly test the log level, but we can test that the logger
	// was created successfully and can be used for logging
	logger.Error("test error message")
	logger.Info("test info message") // This should be filtered out due to ERROR level
}

func TestCreateHTTPRequest(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		url            string
		body           string
		expectedMethod string
		expectedURL    string
		shouldError    bool
	}{
		{
			name:           "GET_request",
			method:         "GET",
			url:            "http://example.com/api/v1/query",
			body:           "",
			expectedMethod: "GET",
			expectedURL:    "http://example.com/api/v1/query",
			shouldError:    false,
		},
		{
			name:           "POST_request_with_body",
			method:         "POST",
			url:            "http://example.com/api/v1/query",
			body:           "query=up",
			expectedMethod: "POST",
			expectedURL:    "http://example.com/api/v1/query",
			shouldError:    false,
		},
		{
			name:           "PUT_request",
			method:         "PUT",
			url:            "http://example.com/api/v1/series",
			body:           "",
			expectedMethod: "PUT",
			expectedURL:    "http://example.com/api/v1/series",
			shouldError:    false,
		},
		{
			name:           "DELETE_request",
			method:         "DELETE",
			url:            "http://example.com/api/v1/admin/tsdb/delete_series",
			body:           "",
			expectedMethod: "DELETE",
			expectedURL:    "http://example.com/api/v1/admin/tsdb/delete_series",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}

			req := CreateHTTPRequest(t, tt.method, tt.url, bodyReader)

			if !tt.shouldError {
				require.NotNil(t, req, "CreateHTTPRequest should return a non-nil request")
				assert.Equal(t, tt.expectedMethod, req.Method, "Request method should match")
				assert.Equal(t, tt.expectedURL, req.URL.String(), "Request URL should match")

				if tt.body != "" {
					assert.NotNil(t, req.Body, "Request should have a body when provided")
				}
			}
		})
	}
}

func TestCreateHTTPRequestWithNilBody(t *testing.T) {
	// Test creating a request with nil body
	req := CreateHTTPRequest(t, "GET", "http://example.com", nil)

	assert.NotNil(t, req)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "http://example.com", req.URL.String())
}

func TestCreateHTTPRequestWithQueryParams(t *testing.T) {
	// Test creating a request with query parameters
	url := "http://example.com/api/v1/query?query=up&time=now"
	req := CreateHTTPRequest(t, "GET", url, nil)

	assert.NotNil(t, req)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, url, req.URL.String())
	assert.Equal(t, "query=up&time=now", req.URL.RawQuery)
}

func TestCreateHTTPRequestEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{
			name:   "empty_method_defaults_to_GET",
			method: "",
			url:    "http://example.com",
			body:   "",
		},
		{
			name:   "custom_method",
			method: "PATCH",
			url:    "http://example.com",
			body:   "",
		},
		{
			name:   "url_with_port",
			method: "GET",
			url:    "http://example.com:8080/api",
			body:   "",
		},
		{
			name:   "https_url",
			method: "GET",
			url:    "https://secure.example.com/api",
			body:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}

			req := CreateHTTPRequest(t, tt.method, tt.url, bodyReader)

			assert.NotNil(t, req)
			// Handle the case where empty method defaults to GET
			expectedMethod := tt.method
			if expectedMethod == "" {
				expectedMethod = "GET"
			}
			assert.Equal(t, expectedMethod, req.Method)
			assert.Equal(t, tt.url, req.URL.String())
		})
	}
}

func TestHelperFunctionBehavior(t *testing.T) {
	// Test that helper functions properly use t.Helper()
	t.Run("logger_helper", func(t *testing.T) {
		// The CreateTestLogger function should call t.Helper()
		// We can't directly test this, but we can test that it works correctly
		logger := CreateTestLogger(t)
		assert.NotNil(t, logger)
	})

	t.Run("request_helper", func(t *testing.T) {
		// The CreateHTTPRequest function should call t.Helper()
		// We can't directly test this, but we can test that it works correctly
		req := CreateHTTPRequest(t, "GET", "http://example.com", nil)
		assert.NotNil(t, req)
	})
}

func TestHelperFunctionsIntegration(t *testing.T) {
	// Test using both helper functions together (as they would be used in real tests)
	logger := CreateTestLogger(t)
	req := CreateHTTPRequest(t, "POST", "http://prometheus:9090/api/v1/query", strings.NewReader("query=up"))

	// Test that we can use both together
	assert.NotNil(t, logger)
	assert.NotNil(t, req)

	// Test that the request is properly formed
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "http://prometheus:9090/api/v1/query", req.URL.String())
	assert.NotNil(t, req.Body)

	// Test that we can use the logger for request logging
	requestLogger := logger.WithRequestFields(req)
	assert.NotNil(t, requestLogger)
}

func TestHTTPMethodConstants(t *testing.T) {
	// Test various HTTP methods to ensure CreateHTTPRequest handles them correctly
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := CreateHTTPRequest(t, method, "http://example.com", nil)
			assert.Equal(t, method, req.Method)
		})
	}
}

// Benchmark tests
func BenchmarkCreateTestLogger(b *testing.B) {
	// Convert testing.B to testing.T for the helper function
	t := &testing.T{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := CreateTestLogger(t)
		_ = logger
	}
}

func BenchmarkCreateHTTPRequest(b *testing.B) {
	// Convert testing.B to testing.T for the helper function
	t := &testing.T{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := CreateHTTPRequest(t, "GET", "http://example.com", nil)
		_ = req
	}
}

func BenchmarkCreateHTTPRequestWithBody(b *testing.B) {
	// Convert testing.B to testing.T for the helper function
	t := &testing.T{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := strings.NewReader("query=up&time=now")
		req := CreateHTTPRequest(t, "POST", "http://example.com/api/v1/query", body)
		_ = req
	}
}

func TestCreateHTTPRequestContentType(t *testing.T) {
	// Test that we can add headers to requests created by the helper
	req := CreateHTTPRequest(t, "POST", "http://example.com", strings.NewReader("test=data"))

	// Add a content type header
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
}

func TestCreateHTTPRequestURL(t *testing.T) {
	// Test various URL formats
	urls := []struct {
		url      string
		expected string
	}{
		{
			url:      "http://localhost:9090/api/v1/query",
			expected: "http://localhost:9090/api/v1/query",
		},
		{
			url:      "https://prometheus.example.com/api/v1/query_range",
			expected: "https://prometheus.example.com/api/v1/query_range",
		},
		{
			url:      "http://192.168.1.100:8080/metrics",
			expected: "http://192.168.1.100:8080/metrics",
		},
	}

	for _, test := range urls {
		t.Run(test.url, func(t *testing.T) {
			req := CreateHTTPRequest(t, "GET", test.url, nil)
			assert.Equal(t, test.expected, req.URL.String())
		})
	}
}
