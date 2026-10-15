package config

import (
	"time"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"
)

// How long each upstream has to answer when several are configured, unless set
const DefaultUpstreamTimeout = 60 * time.Second

type Config struct {
	PrometheusUrls []string
	LogLevel       string
	Port           int
	Client         auth.Client
	// Applies per upstream when several are configured, so that one slow upstream
	// cannot hold up the others. Zero means DefaultUpstreamTimeout
	UpstreamTimeout time.Duration
}
