package config

import (
	"github.com/s-humphreys/prometheus-proxy/internal/auth"
)

type Config struct {
	PrometheusUrl string
	LogLevel      string
	Port          int
	Client        auth.Client
}
