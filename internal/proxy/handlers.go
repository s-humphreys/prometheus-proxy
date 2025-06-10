package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

// Creates an upstream URL for the Prometheus server based on the request,
// including the path and query parameters
func constructPrometheusUrl(logger *slog.Logger, prometheusUrl string, r *http.Request) string {
	upstreamUrl := prometheusUrl + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamUrl = fmt.Sprintf("%s?%s", upstreamUrl, r.URL.RawQuery)
	}
	logger.Debug("constructed upstream prometheus URL", "prometheus_url", upstreamUrl)
	return upstreamUrl
}

// Handles a request which requires authentication. Invokes the implemented clients
// required headers, and forwards the request to the upstream Prometheus server, before
// returning the response to the original client
func authenticatedRequestHandler(logger *logger.Logger, conf *config.Config, pattern string) {
	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		requestId := uuid.New().String()
		cLog := logger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestId,
			"remote_addr", r.RemoteAddr,
		)

		cLog.Info("processing request")

		ctx := r.Context()
		promUrl := constructPrometheusUrl(cLog, conf.PrometheusUrl, r)

		// Copy body if the request method is POST
		var body io.Reader
		if r.Method == http.MethodPost {
			cLog.Debug("copying request body for POST method")
			body = r.Body
		}

		req, err := http.NewRequestWithContext(ctx, r.Method, promUrl, body)
		if err != nil {
			cLog.Error("failed to create upstream request", "error", err)
			http.Error(w, "failed to create upstream request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Copy headers from the original request to the new request
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Add required auth client headers to request
		headers, err := conf.Client.GetHeaders(ctx)
		if err != nil {
			cLog.Error("failed to create client headers", "error", err)
			http.Error(w, "failed to create client headers: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, h := range headers {
			req.Header.Add(h.Key, h.Value)
		}

		// Make the request to the upstream Prometheus server
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cLog.Error("failed to call upstream", "status_code", resp.StatusCode, "error", err)
			http.Error(w, "failed to call upstream: "+err.Error(), resp.StatusCode)
			return
		}
		defer resp.Body.Close()

		// Return response to the original client
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)

		_, err = io.Copy(w, resp.Body)
		if err != nil {
			cLog.Error("failed to copy response body", "error", err)
		}

		cLog.Info("request completed", "status_code", resp.StatusCode)
	})
}
