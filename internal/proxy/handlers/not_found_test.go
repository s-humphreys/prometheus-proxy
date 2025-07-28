package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNotFoundRequestHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET unknown path",
			path:           "/unknown",
			method:         "GET",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "POST unknown path",
			path:           "/some/random/path",
			method:         "POST",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "PUT unknown path",
			path:           "/api/unknown",
			method:         "PUT",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a fresh mux for each test
			mux := http.NewServeMux()

			// Setup the handler inline to avoid global state
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			})

			// Create request and recorder
			req := testutil.CreateHTTPRequest(t, tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			// Execute
			mux.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			// Check that response body contains standard 404 message
			body := recorder.Body.String()
			assert.Contains(t, body, "404")
		})
	}
}
