package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCmdFlags(t *testing.T) {
	var run = func(cmd *cobra.Command, args []string) {}
	originalRun := run
	t.Cleanup(func() {
		run = originalRun
	})
	run = func(_ *cobra.Command, _ []string) {}

	resetCmd := func() {
		prometheusUrls = nil
		logLevel = "INFO"
		port = 9090
		upstreamTimeout = 0
		azureTenantId = ""
		azureClientId = ""
		azureClientSecret = nil

		rootCmd = &cobra.Command{
			Use:     "run",
			Short:   "Starts the proxy",
			Long:    `Starts the proxy server that authenticates requests to a Prometheus instance.`,
			PreRunE: validate,
			Run:     run,
		}

		rootCmd.PersistentFlags().StringSliceVar(&prometheusUrls, "prometheus-url", nil, "The URL of the Prometheus instance to proxy requests to")
		rootCmd.MarkPersistentFlagRequired("prometheus-url")
		rootCmd.PersistentFlags().StringVar(&azureTenantId, "azure-tenant-id", "", "The Azure Tenant ID to use for authentication")
		rootCmd.MarkPersistentFlagRequired("azure-tenant-id")
		rootCmd.PersistentFlags().StringVar(&azureClientId, "azure-client-id", "", "The Azure Client ID to use for authentication")
		rootCmd.MarkPersistentFlagRequired("azure-client-id")
		azureClientSecret = rootCmd.PersistentFlags().String("azure-client-secret", "", "The Azure Client Secret to use for authentication (if not provided, will use Managed Identity)")
		rootCmd.PersistentFlags().IntVar(&port, "port", 9090, "The port to run the proxy on")
		rootCmd.PersistentFlags().DurationVar(&upstreamTimeout, "upstream-timeout", config.DefaultUpstreamTimeout, "How long each Prometheus instance has to respond when several are configured")
		rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO", "The log level to use [DEBUG, INFO]")
	}

	t.Run("SuccessWithAllFlags", func(t *testing.T) {
		resetCmd()
		rootCmd.SetArgs([]string{
			"--prometheus-url", "http://localhost:9090",
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
			"--azure-client-secret", "secret123",
			"--port", "8080",
			"--log-level", "DEBUG",
		})

		err := rootCmd.Execute()

		assert.NoError(t, err)
		assert.Equal(t, []string{"http://localhost:9090"}, prometheusUrls)
		assert.Equal(t, "tenant123", azureTenantId)
		assert.Equal(t, "client123", azureClientId)
		assert.NotNil(t, azureClientSecret)
		assert.Equal(t, "secret123", *azureClientSecret)
		assert.Equal(t, 8080, port)
		assert.Equal(t, "DEBUG", logLevel)
	})

	t.Run("SuccessWithMultiplePrometheusUrls", func(t *testing.T) {
		resetCmd()
		rootCmd.SetArgs([]string{
			"--prometheus-url", "http://a:9090,http://b:9090",
			"--prometheus-url", "http://c:9090",
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
		})

		assert.NoError(t, rootCmd.Execute())
		assert.Equal(t, []string{"http://a:9090", "http://b:9090", "http://c:9090"}, prometheusUrls)
	})

	t.Run("SuccessWithoutOptionalSecret", func(t *testing.T) {
		resetCmd()
		rootCmd.SetArgs([]string{
			"--prometheus-url", "http://localhost:9090",
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
		})

		err := rootCmd.Execute()

		assert.NoError(t, err)
		assert.Equal(t, []string{"http://localhost:9090"}, prometheusUrls)
		assert.Equal(t, "tenant123", azureTenantId)
		assert.Equal(t, "client123", azureClientId)
		// The pointer should exist, but point to the default empty string
		assert.NotNil(t, azureClientSecret)
		assert.Equal(t, "", *azureClientSecret)
	})

	t.Run("UpstreamTimeoutDefaultsAndCanBeSet", func(t *testing.T) {
		args := []string{"--prometheus-url", "http://localhost:9090", "--azure-tenant-id", "t", "--azure-client-id", "c"}

		resetCmd()
		rootCmd.SetArgs(args)
		assert.NoError(t, rootCmd.Execute())
		assert.Equal(t, config.DefaultUpstreamTimeout, upstreamTimeout)

		resetCmd()
		rootCmd.SetArgs(append(args, "--upstream-timeout", "5s"))
		assert.NoError(t, rootCmd.Execute())
		assert.Equal(t, 5*time.Second, upstreamTimeout)
	})

	t.Run("FailureInvalidUpstreamTimeout", func(t *testing.T) {
		resetCmd()
		rootCmd.SetArgs([]string{
			"--prometheus-url", "http://localhost:9090",
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
			"--upstream-timeout", "0s",
		})

		err := rootCmd.Execute()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid upstream timeout")
	})

	t.Run("FailureDuplicatePrometheusUrl", func(t *testing.T) {
		for _, urls := range []string{
			"http://a:9090,http://a:9090",
			"http://a:9090,http://a:9090/",
		} {
			resetCmd()
			rootCmd.SetArgs([]string{
				"--prometheus-url", urls,
				"--azure-tenant-id", "tenant123",
				"--azure-client-id", "client123",
			})

			err := rootCmd.Execute()

			assert.Error(t, err, urls)
			assert.Contains(t, err.Error(), "duplicate prometheus url", urls)
		}
	})

	t.Run("FailureEmptyPrometheusUrl", func(t *testing.T) {
		resetCmd()
		rootCmd.SetArgs([]string{
			"--prometheus-url", "http://a:9090,",
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
		})

		err := rootCmd.Execute()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not be empty")
	})

	t.Run("FailureMissingRequiredFlag", func(t *testing.T) {
		resetCmd()
		// Capture output to avoid polluting test logs
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)

		rootCmd.SetArgs([]string{
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
		})

		err := rootCmd.Execute()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), `required flag(s) "prometheus-url" not set`)
	})

	t.Run("FailureInvalidLogLevel", func(t *testing.T) {
		resetCmd()
		rootCmd.SetArgs([]string{
			"--prometheus-url", "http://localhost:9090",
			"--azure-tenant-id", "tenant123",
			"--azure-client-id", "client123",
			"--log-level", "INVALID",
		})

		err := rootCmd.Execute()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid log level "INVALID"`)
	})
}
