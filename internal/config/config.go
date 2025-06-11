package config

import (
	"os"
	"strconv"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"

	"github.com/go-playground/validator"
)

type EnvConfig struct {
	PrometheusUrl     string `env:"PROMETHEUS_URL" validate:"required"`
	AzureTenantId     string `env:"AZURE_TENANT_ID" validate:"required"`
	AzureClientId     string `env:"AZURE_CLIENT_ID" validate:"required"`
	AzureClientSecret string `env:"AZURE_CLIENT_SECRET"`
}

type Config struct {
	env           *EnvConfig
	PrometheusUrl string
	LogLevel      string
	Port          int
	Client        auth.Client
}

func (c *Config) LoadAzureConfig() {
	c.Client = &auth.AzureClient{
		TenantId:     c.env.AzureTenantId,
		ClientId:     c.env.AzureClientId,
		ClientSecret: c.env.AzureClientSecret,
	}
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

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}

	env := &EnvConfig{
		PrometheusUrl:     os.Getenv("PROMETHEUS_URL"),
		AzureTenantId:     os.Getenv("AZURE_TENANT_ID"),
		AzureClientId:     os.Getenv("AZURE_CLIENT_ID"),
		AzureClientSecret: os.Getenv("AZURE_CLIENT_SECRET"),
	}

	err := validateEnvConfig(env)
	if err != nil {
		return nil, err
	}

	return &Config{
		env:           env,
		PrometheusUrl: env.PrometheusUrl,
		LogLevel:      logLevel,
		Port:          port,
		Client: &auth.AzureClient{
			TenantId:     env.AzureTenantId,
			ClientId:     env.AzureClientId,
			ClientSecret: env.AzureClientSecret,
		},
	}, nil
}
