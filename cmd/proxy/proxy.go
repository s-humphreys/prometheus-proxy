package proxy

import (
	"log"

	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/proxy"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Starts the proxy",
	Long:  `Starts the proxy server that authenticates requests to a Prometheus instance.`,
	Run: func(cmd *cobra.Command, args []string) {
		runProxy()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runProxy() {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config:\n%v", err)
	}

	proxy.Run(conf)
}
