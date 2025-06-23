package handlers

import (
	"net/http"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

// Implements a catch all endpoint to log calls to unimplemented paths
func NotFoundRequestHandler(appLogger *logger.Logger) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		l := appLogger.WithRequestFields(r)
		l.Info("processing unimplemented path request")
		http.NotFound(w, r)
		l.Info("request completed", "status_code", http.StatusNotFound)
	})
}
