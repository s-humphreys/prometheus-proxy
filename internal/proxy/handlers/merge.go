package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

var errInvalidJSON = errors.New("invalid JSON response")

// The outcome of a request to a single upstream. Exactly one of err or a
// status and body is meaningful
type upstreamResult struct {
	url    string
	status int
	header http.Header
	body   []byte
	err    error
}

// The parts of a Prometheus API response which are needed to merge it. Data is
// kept raw so that values are never altered
type prometheusResponse struct {
	Status   string          `json:"status"`
	Data     json.RawMessage `json:"data,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

// Merges the data of the successful upstreams, in upstream order
type mergeFunc func(datas []json.RawMessage) (json.RawMessage, error)

// Returns the function used to merge responses for the request path, or nil if the
// endpoint is not merged and is only sent to the first upstream
func mergeFuncFor(path string) mergeFunc {
	switch {
	case path == "/api/v1/query", path == "/api/v1/query_range":
		return mergeQueryData
	case path == "/api/v1/labels", strings.HasPrefix(path, "/api/v1/label/"):
		return mergeLabelData
	case path == "/api/v1/series":
		return mergeSeriesData
	case path == "/api/v1/metadata":
		return mergeMetadataData
	default:
		return nil
	}
}

// Sends every request concurrently and waits for all of them to finish, each within
// the timeout. The results are in the same order as the requests
func fetchUpstreams(reqs []*http.Request, urls []string, timeout time.Duration) []upstreamResult {
	results := make([]upstreamResult, len(reqs))

	var wg sync.WaitGroup
	for i, req := range reqs {
		wg.Go(func() {
			results[i] = callUpstream(req, urls[i], timeout)
		})
	}
	wg.Wait()

	return results
}

// Sends a request to a single upstream and reads the whole response. The url is the
// configured upstream, used to identify it in warnings and logs. The timeout covers
// reading the whole response
func callUpstream(req *http.Request, upstreamUrl string, timeout time.Duration) upstreamResult {
	result := upstreamResult{url: upstreamUrl}

	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		result.err = describeError(err, timeout)
		return result
	}
	defer resp.Body.Close()

	result.status = resp.StatusCode
	result.header = resp.Header
	result.body, err = io.ReadAll(resp.Body)
	if err != nil {
		result.err = describeError(err, timeout)
	}
	return result
}

// Returns a short description of a failed request for use in a warning
func describeError(err error, timeout time.Duration) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("no response within %s", timeout)
	}
	// The URL error repeats the full URL including the query, which is not needed
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// Returns the decoded upstream response, or an error if the upstream is considered
// to have failed: the request errored, the status is not 2xx, the body is not valid
// JSON or the Prometheus status is not "success"
func decodeUpstream(result upstreamResult) (prometheusResponse, error) {
	var resp prometheusResponse
	if result.err != nil {
		return resp, result.err
	}
	if result.status < 200 || result.status > 299 {
		return resp, fmt.Errorf("status %d", result.status)
	}
	if err := json.Unmarshal(result.body, &resp); err != nil {
		return prometheusResponse{}, errInvalidJSON
	}
	if resp.Status != "success" {
		return prometheusResponse{}, fmt.Errorf("prometheus status %q", resp.Status)
	}
	return resp, nil
}

// Merges query and query_range data. Vector and matrix results are concatenated in
// upstream order, any other result type (scalar or string) returns the first
// upstream's data
func mergeQueryData(datas []json.RawMessage) (json.RawMessage, error) {
	type queryData struct {
		ResultType string            `json:"resultType"`
		Result     []json.RawMessage `json:"result"`
	}

	merged := queryData{Result: []json.RawMessage{}}
	for i, data := range datas {
		var q struct {
			ResultType string          `json:"resultType"`
			Result     json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(data, &q); err != nil {
			return nil, fmt.Errorf("unexpected query data: %w", err)
		}

		if i == 0 {
			merged.ResultType = q.ResultType
			if q.ResultType != "vector" && q.ResultType != "matrix" {
				return datas[0], nil
			}
		}
		if q.ResultType != merged.ResultType {
			continue
		}

		var items []json.RawMessage
		if err := json.Unmarshal(q.Result, &items); err != nil {
			return nil, fmt.Errorf("unexpected query result: %w", err)
		}
		merged.Result = append(merged.Result, items...)
	}
	return json.Marshal(merged)
}

// Merges labels and label values data into a sorted list without duplicates
func mergeLabelData(datas []json.RawMessage) (json.RawMessage, error) {
	all := []string{}
	for _, data := range datas {
		var values []string
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("unexpected label data: %w", err)
		}
		all = append(all, values...)
	}
	slices.Sort(all)
	return json.Marshal(slices.Compact(all))
}

// Merges series data into a list of label sets without duplicates, sorted so that the
// result is deterministic
func mergeSeriesData(datas []json.RawMessage) (json.RawMessage, error) {
	all := []json.RawMessage{}
	seen := map[string]struct{}{}
	for _, data := range datas {
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("unexpected series data: %w", err)
		}
		for _, item := range items {
			if _, ok := seen[string(item)]; !ok {
				seen[string(item)] = struct{}{}
				all = append(all, item)
			}
		}
	}
	slices.SortFunc(all, func(a, b json.RawMessage) int { return bytes.Compare(a, b) })
	return json.Marshal(all)
}

// Merges metadata data into a map of metric name to the distinct metadata of that
// metric, in upstream order
func mergeMetadataData(datas []json.RawMessage) (json.RawMessage, error) {
	all := map[string][]json.RawMessage{}
	seen := map[string]struct{}{}
	for _, data := range datas {
		var byMetric map[string][]json.RawMessage
		if err := json.Unmarshal(data, &byMetric); err != nil {
			return nil, fmt.Errorf("unexpected metadata: %w", err)
		}
		for metric, items := range byMetric {
			for _, item := range items {
				key := metric + "|" + string(item) // metric names cannot contain "|"
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					all[metric] = append(all[metric], item)
				}
			}
		}
	}
	return json.Marshal(all)
}
