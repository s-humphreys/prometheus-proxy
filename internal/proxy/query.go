package proxy

import (
	"io"
	"net/http"

	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"

	"github.com/google/uuid"
)

// Handles the `/api/v1/query` endpoint by forwarding requests to the upstream Prometheus server
func queryHandler(logger *logger.Logger, conf *config.Config) {
	http.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		requestId := uuid.New().String()
		cLog := logger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestId,
			"remote_addr", r.RemoteAddr,
		)

		cLog.Info("processing request")

		if r.Method != http.MethodGet {
			cLog.Error("invalid request method")
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		ctx := r.Context()
		promUrl := constructPrometheusUrl(conf.PrometheusUrl, r)
		req, err := http.NewRequestWithContext(ctx, "GET", promUrl, nil)
		if err != nil {
			cLog.Error("failed to create upstream request", "error", err)
			http.Error(w, "failed to create upstream request: "+err.Error(), http.StatusInternalServerError)
			return
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
