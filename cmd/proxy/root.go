package proxy

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "prometheus-proxy",
	Short: "prometheus-proxy is a proxy that authenticates requests to a Prometheus instance",
	Run:   func(cmd *cobra.Command, args []string) {},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}
}
