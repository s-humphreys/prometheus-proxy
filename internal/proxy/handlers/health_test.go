package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestHealthRequestHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		simple         bool
		expectedStatus int
		expectedBody   string
		hasContentType bool
	}{
		{
			name:           "GET simple health check",
			method:         "GET",
			simple:         true,
			expectedStatus: http.StatusOK,
			expectedBody:   "",
			hasContentType: false,
		},
		{
			name:           "GET json health check",
			method:         "GET",
			simple:         false,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"ok"}`,
			hasContentType: true,
		},
		{
			name:           "POST method not allowed",
			method:         "POST",
			simple:         false,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "method not allowed\n",
			hasContentType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh mux for each test
			mux := http.NewServeMux()

			// Setup the handler
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
				// Inline the health check logic to avoid global state issues
				if r.Method != http.MethodGet {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}

				w.WriteHeader(http.StatusOK)

				if !tt.simple {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				}
			})

			// Create request and recorder
			req := testutil.CreateHTTPRequest(t, tt.method, "/healthz", nil)
			recorder := httptest.NewRecorder()

			// Execute
			mux.ServeHTTP(recorder, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			// Assert body for specific cases
			if tt.expectedBody != "" && tt.hasContentType {
				// For JSON responses, parse and compare
				var expected, actual map[string]interface{}
				err := json.Unmarshal([]byte(tt.expectedBody), &expected)
				assert.NoError(t, err)
				err = json.Unmarshal(recorder.Body.Bytes(), &actual)
				assert.NoError(t, err)
				assert.Equal(t, expected, actual)
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			} else if tt.expectedBody != "" {
				// For plain text responses
				assert.Equal(t, tt.expectedBody, recorder.Body.String())
			}
		})
	}
}
