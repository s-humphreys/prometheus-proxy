package handlers

import (
	"bytes"
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
	if r.Method == http.MethodGet && r.URL.RawQuery != "" {
		upstreamUrl = fmt.Sprintf("%s?%s", upstreamUrl, r.URL.RawQuery)
	}
	logger.Debug("constructed upstream prometheus URL", "prometheus_url", upstreamUrl)
	return upstreamUrl
}

// Returns a copy of the provided HTTP headers with sensitive information removed
func redactedHeaders(header http.Header) http.Header {
	filteredHeader := make(http.Header)

	for k, v := range header {
		if k == "Authorization" || k == "Cookie" {
			filteredHeader.Set(k, "[REDACTED]")
		} else {
			filteredHeader[k] = v
		}
	}

	return filteredHeader
}

// Handles a request which requires authentication. Invokes the implemented clients
// required headers, and forwards the request to the upstream Prometheus server, before
// returning the response to the original client
func PrometheusRequestHandler(logger *logger.Logger, conf *config.Config, pattern string) {
	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		requestId := uuid.New().String()
		l := logger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestId,
			"remote_addr", r.RemoteAddr,
		)

		l.Info("processing request")
		ctx := r.Context()
		promUrl := constructPrometheusUrl(l, conf.PrometheusUrl, r)

		// Copy body if the request method is POST & store for logging/forwarding
		var bodyForUpstream io.Reader
		var requestBodyBytes []byte

		if r.Method == http.MethodPost {
			var errReadBody error
			requestBodyBytes, errReadBody = io.ReadAll(r.Body)
			if errReadBody != nil {
				l.Error("failed to read request body for logging", "error", errReadBody)
				http.Error(w, "failed to read request body: "+errReadBody.Error(), http.StatusInternalServerError)
				return
			}

			l.Debug("copying request body for POST method")
			bodyForUpstream = bytes.NewBuffer(requestBodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, r.Method, promUrl, bodyForUpstream)
		if err != nil {
			l.Error("failed to create upstream request", "error", err)
			http.Error(w, "failed to create upstream request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodPost && bodyForUpstream != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		// Add required auth client headers to request
		headers, err := conf.Client.GetHeaders(ctx)
		if err != nil {
			l.Error("failed to create client headers", "error", err)
			http.Error(w, "failed to create client headers: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for _, h := range headers {
			req.Header.Add(h.Key, h.Value)
		}

		l.Debug("forwarding request to upstream prometheus",
			"url", promUrl,
			"method", r.Method,
			"headers", redactedHeaders(req.Header),
			"body", string(requestBodyBytes),
		)

		// Make the request to the upstream Prometheus server
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			l.Error("failed to call upstream", "status_code", resp.StatusCode, "error", err)
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
			l.Error("failed to copy response body", "error", err)
		}

		l.Info("request completed", "status_code", resp.StatusCode)
	})
}
