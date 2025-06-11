package proxy

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

var errForbiddenMethod = errors.New("forbidden request method")

// Run starts the HTTP server and listens for incoming requests
func Run(c *config.Config) {
	l, err := logger.New(c.LogLevel)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}

	c.Client.InitClient(l)

	runtimeInfo := newruntimeInfoData()
	buildInfo := newbuildInfoData()

	// Setup handlers for routes
	healthRequestHandler(l)
	mockStatusConfigHandler(l)
	mockStatusRuntimeInfoHandler(l, runtimeInfo)
	mockStatusBuildInfoHandler(l, buildInfo)
	authenticatedRequestHandler(l, c, "/api/v1/query")
	authenticatedRequestHandler(l, c, "/api/v1/query_range")
	authenticatedRequestHandler(l, c, "/api/v1/format_query")
	authenticatedRequestHandler(l, c, "/api/v1/parse_query")
	authenticatedRequestHandler(l, c, "/api/v1/series")
	authenticatedRequestHandler(l, c, "/api/v1/labels")
	authenticatedRequestHandler(l, c, "/api/v1/metadata")
	authenticatedRequestHandler(l, c, "/api/v1/status/flags") // TODO: Implement mock handler like runtimeinfo

	addr := fmt.Sprintf(":%d", c.Port)
	l.Info("starting prometheus proxy", "listening", addr, "port", c.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
