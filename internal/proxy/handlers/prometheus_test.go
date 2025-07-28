package handlers

import (
	"net/http"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestConstructPrometheusURL(t *testing.T) {
	t.Parallel()
	logger := testutil.CreateTestLogger(t)

	tests := []struct {
		name          string
		prometheusUrl string
		requestURL    string
		method        string
		expected      string
	}{
		{
			name:          "GET with query parameters",
			prometheusUrl: "http://prometheus:9090",
			requestURL:    "/api/v1/query?query=up&time=now",
			method:        "GET",
			expected:      "http://prometheus:9090/api/v1/query?query=up&time=now",
		},
		{
			name:          "POST without query parameters",
			prometheusUrl: "http://prometheus:9090",
			requestURL:    "/api/v1/query",
			method:        "POST",
			expected:      "http://prometheus:9090/api/v1/query",
		},
		{
			name:          "GET without query parameters",
			prometheusUrl: "http://prometheus:9090",
			requestURL:    "/api/v1/labels",
			method:        "GET",
			expected:      "http://prometheus:9090/api/v1/labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := testutil.CreateHTTPRequest(t, tt.method, tt.requestURL, nil)
			result := constructPrometheusURL(logger, tt.prometheusUrl, req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactedHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    http.Header
		expected http.Header
	}{
		{
			name: "redact authorization header",
			input: http.Header{
				"Authorization": []string{"Bearer secret-token"},
				"Content-Type":  []string{"application/json"},
			},
			expected: http.Header{
				"Authorization": []string{"[REDACTED]"},
				"Content-Type":  []string{"application/json"},
			},
		},
		{
			name: "redact cookie header",
			input: http.Header{
				"Cookie":     []string{"session=secret"},
				"User-Agent": []string{"test-agent"},
			},
			expected: http.Header{
				"Cookie":     []string{"[REDACTED]"},
				"User-Agent": []string{"test-agent"},
			},
		},
		{
			name: "no sensitive headers",
			input: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"test-agent"},
			},
			expected: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"test-agent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := redactedHeaders(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
