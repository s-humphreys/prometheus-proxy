package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

// Implements a health check endpoint that returns a simple JSON response.
// Set simple to true to return a simple 200 response without content
func HealthRequestHandler(appLogger *logger.Logger, url string, simple bool) {
	http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		l := appLogger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestID,
			"remote_addr", r.RemoteAddr,
		)

		l.Debug("processing health check request")

		if r.Method != http.MethodGet {
			l.Warn("health check received non-GET request")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.WriteHeader(http.StatusOK)

		if simple {
			l.Debug("request completed", "status_code", http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		l.Debug("request completed", "status_code", http.StatusOK)
	})
}
