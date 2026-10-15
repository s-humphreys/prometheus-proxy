package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"
	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

// Creates an upstream URL for the Prometheus server based on the request,
// including the path and query parameters
func constructPrometheusURL(logger *logger.Logger, prometheusUrl string, r *http.Request) string {
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

// Creates a request to an upstream Prometheus server from the original request, with
// the same method, path, query and body, and the auth client headers added
func newUpstreamRequest(l *logger.Logger, prometheusUrl string, r *http.Request, body []byte, headers []auth.ClientHeader) (*http.Request, error) {
	promUrl := constructPrometheusURL(l, prometheusUrl, r)

	var bodyForUpstream io.Reader
	if r.Method == http.MethodPost {
		bodyForUpstream = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, promUrl, bodyForUpstream)
	if err != nil {
		return nil, err
	}

	if r.Method == http.MethodPost && bodyForUpstream != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Add required auth client headers to request
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}

	return req, nil
}

// Handles a request which requires authentication. Invokes the implemented clients
// required headers, and forwards the request to the upstream Prometheus server, before
// returning the response to the original client. If several upstreams are configured,
// the endpoints which can be merged are sent to all of them and the responses merged,
// any other endpoint is sent to the first upstream only
func PrometheusRequestHandler(logger *logger.Logger, conf *config.Config, pattern string) {
	http.HandleFunc(pattern, newPrometheusHandler(logger, conf))
}

func newPrometheusHandler(logger *logger.Logger, conf *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		l := logger.WithRequestFields(r)
		l.Info("processing request")

		ctx := r.Context()

		if len(conf.PrometheusUrls) == 0 {
			l.Error("no prometheus urls configured")
			http.Error(w, "no prometheus urls configured", http.StatusInternalServerError)
			return
		}

		// Copy body if the request method is POST & store for logging/forwarding
		var requestBodyBytes []byte

		if r.Method == http.MethodPost {
			var errReadBody error
			requestBodyBytes, errReadBody = io.ReadAll(r.Body)
			if errReadBody != nil {
				l.Error("failed to read request body for logging", "error", errReadBody)
				http.Error(w, "failed to read request body: "+errReadBody.Error(), http.StatusInternalServerError)
				return
			}

			if len(requestBodyBytes) == 0 && r.URL.RawQuery != "" {
				l.Debug("got request with empty body and URL query parameters, using URL query as body.", "query", r.URL.RawQuery)
				requestBodyBytes = []byte(r.URL.RawQuery)
			}

			l.Debug("copying request body for POST method")
		}

		// Fetch the required auth client headers once, they are valid for every upstream
		headers, err := conf.Client.GetHeaders(ctx)
		if err != nil {
			l.Error("failed to create client headers", "error", err)
			http.Error(w, "failed to create client headers: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Endpoints which are not merged only go to the first upstream
		urls := conf.PrometheusUrls
		merge := mergeFuncFor(r.URL.Path)
		if merge == nil {
			urls = urls[:1]
		}

		reqs := make([]*http.Request, len(urls))
		for i, promUrl := range urls {
			reqs[i], err = newUpstreamRequest(l, promUrl, r, requestBodyBytes, headers)
			if err != nil {
				l.Error("failed to create upstream request", "error", err)
				http.Error(w, "failed to create upstream request: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		l.Info("forwarding request to upstream prometheus",
			"prometheus_urls", urls,
			"headers", redactedHeaders(reqs[0].Header),
			"body", string(requestBodyBytes),
		)

		if len(reqs) > 1 {
			timeout := conf.UpstreamTimeout
			if timeout <= 0 {
				timeout = config.DefaultUpstreamTimeout
			}
			writeMergedResponse(l, w, merge, fetchUpstreams(reqs, urls, timeout))
			return
		}

		// Make the request to the upstream Prometheus server
		resp, err := http.DefaultClient.Do(reqs[0])
		if err != nil {
			l.Error("failed to call upstream", "error", err)
			http.Error(w, "failed to call upstream: "+err.Error(), http.StatusBadGateway)
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
	}
}

// Merges the upstream responses and returns them to the original client. Upstreams
// which failed are reported in the warnings of the response, followed by the warnings
// of the upstreams which succeeded. If every upstream failed,
// the response of the first one which answered with an error status is returned as is
// so that the client sees Prometheus's own error, otherwise it is a bad gateway
func writeMergedResponse(l *logger.Logger, w http.ResponseWriter, merge mergeFunc, results []upstreamResult) {
	var datas []json.RawMessage
	var failures, upstreamWarnings []string
	for _, result := range results {
		resp, err := decodeUpstream(result)
		if err != nil {
			l.Warn("upstream failed", "prometheus_url", result.url, "error", err)
			failures = append(failures, fmt.Sprintf("upstream %s failed: %v", result.url, err))
			continue
		}
		datas = append(datas, resp.Data)
		for _, warning := range resp.Warnings {
			upstreamWarnings = append(upstreamWarnings, fmt.Sprintf("upstream %s: %s", result.url, warning))
		}
	}

	if len(datas) == 0 {
		for _, result := range results {
			if result.err == nil && (result.status < 200 || result.status > 299) {
				l.Info("request completed", "status_code", result.status)
				w.Header().Set("Content-Type", result.header.Get("Content-Type"))
				w.WriteHeader(result.status)
				_, _ = w.Write(result.body)
				return
			}
		}
		l.Info("request completed", "status_code", http.StatusBadGateway)
		http.Error(w, "failed to call upstream: "+strings.Join(failures, "; "), http.StatusBadGateway)
		return
	}

	data, err := merge(datas)
	if err != nil {
		l.Error("failed to merge upstream responses", "error", err)
		http.Error(w, "failed to merge upstream responses: "+err.Error(), http.StatusBadGateway)
		return
	}

	body, err := json.Marshal(prometheusResponse{Status: "success", Data: data, Warnings: append(failures, upstreamWarnings...)})
	if err != nil {
		l.Error("failed to encode merged response", "error", err)
		http.Error(w, "failed to encode merged response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write(body); err != nil {
		l.Error("failed to write response body", "error", err)
	}

	l.Info("request completed", "status_code", http.StatusOK, "failed_upstreams", len(failures))
}
