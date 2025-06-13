package proxy

import (
	"fmt"
	"log"
	"net/http"

	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
	"github.com/s-humphreys/prometheus-proxy/internal/proxy/handlers"
)

// Run starts the HTTP server and listens for incoming requests
func Run(c *config.Config) {
	l, err := logger.New(c.LogLevel)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}

	err = c.Client.InitClient(l)
	if err != nil {
		log.Fatalf("failed to initialize authentication client: %v", err)
	}

	runtimeInfo := handlers.NewRuntimeInfoData()
	buildInfo := handlers.NewBuildInfoData()

	// Setup handlers for routes
	handlers.HealthRequestHandler(l)
	handlers.MockStatusConfigHandler(l)
	handlers.MockStatusRuntimeInfoHandler(l, runtimeInfo)
	handlers.MockStatusBuildInfoHandler(l, buildInfo)
	handlers.PrometheusRequestHandler(l, c, "/api/v1/query")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/query_range")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/format_query")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/parse_query")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/series")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/labels")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/metadata")
	handlers.PrometheusRequestHandler(l, c, "/api/v1/status/flags") // TODO: Implement mock handler like runtimeinfo

	addr := fmt.Sprintf(":%d", c.Port)
	l.Info("starting prometheus proxy", "listening", addr, "port", c.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
