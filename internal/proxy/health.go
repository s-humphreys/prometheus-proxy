package proxy

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

// Implements a health check endpoint that returns a simple JSON response
func healthRequestHandler(appLogger *logger.Logger) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		cLog := appLogger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestID,
			"remote_addr", r.RemoteAddr,
		)

		cLog.Debug("processing health check request")

		if r.Method != http.MethodGet {
			cLog.Warn("health check received non-GET request")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		cLog.Debug("health check successful")
	})
}
