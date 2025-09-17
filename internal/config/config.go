package config

import (
	"os"
	"strconv"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator"
)

type EnvConfig struct {
	PrometheusUrl     string `env:"PROMETHEUS_URL" validate:"required"`
	AzureTenantId     string `env:"AZURE_TENANT_ID" validate:"required"`
	AzureClientId     string `env:"AZURE_CLIENT_ID" validate:"required"`
	AzureClientSecret string `env:"AZURE_CLIENT_SECRET"`
	LogLevel          string `env:"LOG_LEVEL"`
}

type Config struct {
	env           *EnvConfig
	PrometheusUrl string
	LogLevel      string
	Port          int
	Client        auth.Client
}

func validateEnvConfig(c *EnvConfig) error {
	validate := validator.New()
	return validate.Struct(c)
}

func LoadConfig() (*Config, error) {
	port := 9090
	if p := os.Getenv("PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	ec := &EnvConfig{}
	if err := env.Parse(ec); err != nil {
		return nil, err
	}

	err := validateEnvConfig(ec)
	if err != nil {
		return nil, err
	}

	if ec.LogLevel == "" {
		ec.LogLevel = "INFO"
	}

	return &Config{
		env:           ec,
		PrometheusUrl: ec.PrometheusUrl,
		LogLevel:      ec.LogLevel,
		Port:          port,
		Client: &auth.AzureClient{
			TenantId:     ec.AzureTenantId,
			ClientId:     ec.AzureClientId,
			ClientSecret: ec.AzureClientSecret,
		},
	}, nil
}
