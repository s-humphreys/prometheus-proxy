package main

import (
	"fmt"
	"log"
	"maps"
	"os"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"
	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
	"github.com/s-humphreys/prometheus-proxy/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	rootCmd           *cobra.Command
	prometheusUrl     string
	logLevel          string
	port              int
	azureTeneantId    string
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

	rootCmd.PersistentFlags().StringVar(&prometheusUrl, "prometheus-url", "", "The URL of the Prometheus instance to proxy requests to")
	rootCmd.MarkPersistentFlagRequired("prometheus-url")
	rootCmd.PersistentFlags().StringVar(&azureTeneantId, "azure-tenant-id", "", "The Azure Tenant ID to use for authentication")
	rootCmd.MarkPersistentFlagRequired("azure-tenant-id")
	rootCmd.PersistentFlags().StringVar(&azureClientId, "azure-client-id", "", "The Azure Client ID to use for authentication")
	rootCmd.MarkPersistentFlagRequired("azure-client-id")
	azureClientSecret = rootCmd.PersistentFlags().String("azure-client-secret", "", "The Azure Client Secret to use for authentication (if not provided, will use Managed Identity)")
	rootCmd.PersistentFlags().IntVar(&port, "port", 9090, "The port to run the proxy on")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO", "The log level to use@")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}
}

func validate(_ *cobra.Command, _ []string) error {
	if _, exists := logger.LogLevelMap[logLevel]; !exists {
		return fmt.Errorf("invalid log level %q, allowed values are: %v", logLevel, maps.Keys(logger.LogLevelMap))
	}
	return nil
}

func run(_ *cobra.Command, _ []string) {
	var secret *string
	if rootCmd.Flags().Changed("azure-client-secret") {
		secret = azureClientSecret
	}

	conf := &config.Config{
		PrometheusUrl: prometheusUrl,
		LogLevel:      logLevel,
		Port:          port,
		Client: &auth.AzureClient{
			TenantId:     azureTeneantId,
			ClientId:     azureClientId,
			ClientSecret: secret,
		},
	}

	proxy.Run(conf)
}
