package main

import (
	"fmt"
	"log"
	"maps"
	"os"
	"time"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"
	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
	"github.com/s-humphreys/prometheus-proxy/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	rootCmd           *cobra.Command
	prometheusUrls    []string
	logLevel          string
	port              int
	upstreamTimeout   time.Duration
	azureTenantId     string
	azureClientId     string
	azureClientSecret *string
)

func main() {
	rootCmd = &cobra.Command{
		Use:     "run",
		Short:   "Starts the proxy",
		Long:    `Starts the proxy server that authenticates requests to a Prometheus instance.`,
		PreRunE: validate,
		Run:     run,
	}

	rootCmd.PersistentFlags().StringSliceVar(&prometheusUrls, "prometheus-url", nil, "The URL of the Prometheus instance to proxy requests to, repeat or comma-separate to aggregate several instances")
	rootCmd.MarkPersistentFlagRequired("prometheus-url")
	rootCmd.PersistentFlags().StringVar(&azureTenantId, "azure-tenant-id", "", "The Azure Tenant ID to use for authentication")
	rootCmd.MarkPersistentFlagRequired("azure-tenant-id")
	rootCmd.PersistentFlags().StringVar(&azureClientId, "azure-client-id", "", "The Azure Client ID to use for authentication")
	rootCmd.MarkPersistentFlagRequired("azure-client-id")
	azureClientSecret = rootCmd.PersistentFlags().String("azure-client-secret", "", "The Azure Client Secret to use for authentication (if not provided, will use Managed Identity)")
	rootCmd.PersistentFlags().IntVar(&port, "port", 9090, "The port to run the proxy on")
	rootCmd.PersistentFlags().DurationVar(&upstreamTimeout, "upstream-timeout", config.DefaultUpstreamTimeout, "How long each Prometheus instance has to respond when several are configured")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO", "The log level to use")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}
}

func validate(_ *cobra.Command, _ []string) error {
	if _, exists := logger.LogLevelMap[logLevel]; !exists {
		return fmt.Errorf("invalid log level %q, allowed values are: %v", logLevel, maps.Keys(logger.LogLevelMap))
	}
	if upstreamTimeout <= 0 {
		return fmt.Errorf("invalid upstream timeout %s, it must be greater than zero", upstreamTimeout)
	}
	return nil
}

func run(_ *cobra.Command, _ []string) {
	var secret *string
	if rootCmd.Flags().Changed("azure-client-secret") {
		secret = azureClientSecret
	}

	conf := &config.Config{
		PrometheusUrls:  prometheusUrls,
		LogLevel:        logLevel,
		Port:            port,
		UpstreamTimeout: upstreamTimeout,
		Client: &auth.AzureClient{
			TenantId:     azureTenantId,
			ClientId:     azureClientId,
			ClientSecret: secret,
		},
	}

	proxy.Run(conf)
}
