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

// Creates an upstream URL for the Prometheus server based on the request,
// including the path and query parameters
func constructPrometheusUrl(prometheusUrl string, r *http.Request) string {
	upstreamUrl := prometheusUrl + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamUrl = fmt.Sprintf("%s?%s", upstreamUrl, r.URL.RawQuery)
	}
	return upstreamUrl
}

// Run starts the HTTP server and listens for incoming requests
func Run(c *config.Config) {
	l, err := logger.New(c.LogLevel)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	c.Client.SetLogger(l)
	c.Client.InitClient()

	queryHandler(l, c)

	addr := fmt.Sprintf(":%d", c.Port)
	l.Info("starting prometheus proxy", "listening", addr, "port", c.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
