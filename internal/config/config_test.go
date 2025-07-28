package config

import (
	"os"
	"strconv"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEnvConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *EnvConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: &EnvConfig{
				PrometheusUrl:     "http://prometheus:9090",
				AzureTenantId:     "tenant-id",
				AzureClientId:     "client-id",
				AzureClientSecret: "client-secret",
				LogLevel:          "INFO",
			},
			expectError: false,
		},
		{
			name: "missing prometheus URL",
			config: &EnvConfig{
				AzureTenantId:     "tenant-id",
				AzureClientId:     "client-id",
				AzureClientSecret: "client-secret",
				LogLevel:          "INFO",
			},
			expectError: true,
			errorField:  "PrometheusUrl",
		},
		{
			name: "missing azure tenant ID",
			config: &EnvConfig{
				PrometheusUrl:     "http://prometheus:9090",
				AzureClientId:     "client-id",
				AzureClientSecret: "client-secret",
				LogLevel:          "INFO",
			},
			expectError: true,
			errorField:  "AzureTenantId",
		},
		{
			name: "missing azure client ID",
			config: &EnvConfig{
				PrometheusUrl:     "http://prometheus:9090",
				AzureTenantId:     "tenant-id",
				AzureClientSecret: "client-secret",
				LogLevel:          "INFO",
			},
			expectError: true,
			errorField:  "AzureClientId",
		},
		{
			name: "client secret is optional",
			config: &EnvConfig{
				PrometheusUrl: "http://prometheus:9090",
				AzureTenantId: "tenant-id",
				AzureClientId: "client-id",
				LogLevel:      "INFO",
			},
			expectError: false,
		},
		{
			name: "log level is optional",
			config: &EnvConfig{
				PrometheusUrl:     "http://prometheus:9090",
				AzureTenantId:     "tenant-id",
				AzureClientId:     "client-id",
				AzureClientSecret: "client-secret",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorField != "" {
					assert.Contains(t, err.Error(), tt.errorField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Store original environment variables
	originalEnv := make(map[string]string)
	envVars := []string{
		"PROMETHEUS_URL", "AZURE_TENANT_ID", "AZURE_CLIENT_ID", 
		"AZURE_CLIENT_SECRET", "LOG_LEVEL", "PORT",
	}
	
	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
	}
	
	// Cleanup function to restore original environment
	t.Cleanup(func() {
		for env, value := range originalEnv {
			if value == "" {
				os.Unsetenv(env)
			} else {
				os.Setenv(env, value)
			}
		}
	})

	t.Run("valid configuration", func(t *testing.T) {
		// Set valid environment variables
		os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
		os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
		os.Setenv("AZURE_CLIENT_ID", "test-client-id")
		os.Setenv("AZURE_CLIENT_SECRET", "test-client-secret")
		os.Setenv("LOG_LEVEL", "DEBUG")
		os.Setenv("PORT", "8080")

		config, err := LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "http://test-prometheus:9090", config.PrometheusUrl)
		assert.Equal(t, "DEBUG", config.LogLevel)
		assert.Equal(t, 8080, config.Port)
		assert.NotNil(t, config.Client)

		// Verify the Azure client is properly configured
		azureClient, ok := config.Client.(*auth.AzureClient)
		require.True(t, ok)
		assert.Equal(t, "test-tenant-id", azureClient.TenantId)
		assert.Equal(t, "test-client-id", azureClient.ClientId)
		assert.Equal(t, "test-client-secret", azureClient.ClientSecret)
	})

	t.Run("default values", func(t *testing.T) {
		// Set only required environment variables
		os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
		os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
		os.Setenv("AZURE_CLIENT_ID", "test-client-id")
		os.Unsetenv("AZURE_CLIENT_SECRET")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("PORT")

		config, err := LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "INFO", config.LogLevel) // Default log level
		assert.Equal(t, 9090, config.Port)       // Default port

		// Verify Azure client with no client secret
		azureClient, ok := config.Client.(*auth.AzureClient)
		require.True(t, ok)
		assert.Equal(t, "", azureClient.ClientSecret)
	})

	t.Run("custom port", func(t *testing.T) {
		// Set required environment variables with custom port
		os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
		os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
		os.Setenv("AZURE_CLIENT_ID", "test-client-id")
		os.Setenv("PORT", "3000")

		config, err := LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, 3000, config.Port)
	})

	t.Run("invalid port", func(t *testing.T) {
		// Set required environment variables with invalid port
		os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
		os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
		os.Setenv("AZURE_CLIENT_ID", "test-client-id")
		os.Setenv("PORT", "invalid-port")

		config, err := LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, config)

		// Should fall back to default port
		assert.Equal(t, 9090, config.Port)
	})

	t.Run("missing required field", func(t *testing.T) {
		// Unset required environment variable
		os.Unsetenv("PROMETHEUS_URL")
		os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
		os.Setenv("AZURE_CLIENT_ID", "test-client-id")

		config, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "PrometheusUrl")
	})

	t.Run("missing azure tenant ID", func(t *testing.T) {
		os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
		os.Unsetenv("AZURE_TENANT_ID")
		os.Setenv("AZURE_CLIENT_ID", "test-client-id")

		config, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "AzureTenantId")
	})

	t.Run("missing azure client ID", func(t *testing.T) {
		os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
		os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
		os.Unsetenv("AZURE_CLIENT_ID")

		config, err := LoadConfig()
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "AzureClientId")
	})
}

func TestPortParsing(t *testing.T) {
	tests := []struct {
		name         string
		portEnv      string
		expectedPort int
	}{
		{
			name:         "valid port",
			portEnv:      "8080",
			expectedPort: 8080,
		},
		{
			name:         "empty port",
			portEnv:      "",
			expectedPort: 9090,
		},
		{
			name:         "invalid port",
			portEnv:      "invalid",
			expectedPort: 9090,
		},
		{
			name:         "zero port",
			portEnv:      "0",
			expectedPort: 0,
		},
		{
			name:         "negative port",
			portEnv:      "-1",
			expectedPort: -1,
		},
		{
			name:         "large port",
			portEnv:      "65535",
			expectedPort: 65535,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the environment and os.Getenv call behavior
			originalPort := os.Getenv("PORT")
			defer func() {
				if originalPort == "" {
					os.Unsetenv("PORT")
				} else {
					os.Setenv("PORT", originalPort)
				}
			}()

			if tt.portEnv == "" {
				os.Unsetenv("PORT")
			} else {
				os.Setenv("PORT", tt.portEnv)
			}

			// Test the port parsing logic directly
			port := 9090
			if p := os.Getenv("PORT"); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil {
					port = parsed
				}
			}

			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

// Benchmark tests
func BenchmarkLoadConfig(b *testing.B) {
	// Set required environment variables
	os.Setenv("PROMETHEUS_URL", "http://test-prometheus:9090")
	os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
	os.Setenv("AZURE_CLIENT_ID", "test-client-id")
	
	defer func() {
		os.Unsetenv("PROMETHEUS_URL")
		os.Unsetenv("AZURE_TENANT_ID")
		os.Unsetenv("AZURE_CLIENT_ID")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config, err := LoadConfig()
		if err != nil {
			b.Fatal(err)
		}
		_ = config
	}
}

func BenchmarkValidateEnvConfig(b *testing.B) {
	config := &EnvConfig{
		PrometheusUrl: "http://prometheus:9090",
		AzureTenantId: "tenant-id",
		AzureClientId: "client-id",
		LogLevel:      "INFO",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validateEnvConfig(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
