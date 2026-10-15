package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/s-humphreys/prometheus-proxy/internal/auth"
	"github.com/s-humphreys/prometheus-proxy/internal/config"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
	"github.com/s-humphreys/prometheus-proxy/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testAuthClient struct {
	headersError error
}

func (c *testAuthClient) InitClient(_ *logger.Logger) error { return nil }

func (c *testAuthClient) AcquireToken(_ context.Context) (string, error) {
	return "test-token", nil
}

func (c *testAuthClient) GetHeaders(_ context.Context) ([]auth.ClientHeader, error) {
	return []auth.ClientHeader{{Key: "Authorization", Value: "Bearer test-token"}}, c.headersError
}

// A fake upstream which answers every request the same way and records them
type fakeUpstream struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string // "METHOD path?query body" of each request
	auth     []string
}

func newUpstream(t *testing.T, status int, body string) *fakeUpstream {
	t.Helper()
	f := &fakeUpstream{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI()+" "+string(b))
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(f.Close)
	return f
}

// The requests and Authorization headers received so far
func (f *fakeUpstream) received() (requests, auth []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests), slices.Clone(f.auth)
}

// The URL of an upstream which has been shut down, so connecting to it fails
func unreachableURL() string {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

func success(data string) string { return `{"status":"success","data":` + data + `}` }

// Sends a request through the handler configured with the given upstream urls
func serve(t *testing.T, urls []string, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveWith(t, &config.Config{PrometheusUrls: urls, Client: &testAuthClient{}}, method, target, body)
}

func serveWith(t *testing.T, conf *config.Config, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := newPrometheusHandler(testutil.CreateTestLogger(t), conf)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

type merged struct {
	Status   string          `json:"status"`
	Data     json.RawMessage `json:"data"`
	Warnings []string        `json:"warnings"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) merged {
	t.Helper()
	var m merged
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m), rec.Body.String())
	return m
}

func TestMergeQueryConcatenatesInUpstreamOrder(t *testing.T) {
	t.Parallel()
	a := newUpstream(t, 200, success(`{"resultType":"vector","result":[{"metric":{"cluster":"a"},"value":[1,"1"]}]}`))
	b := newUpstream(t, 200, success(`{"resultType":"vector","result":[{"metric":{"cluster":"b"},"value":[1,"2"]}]}`))

	rec := serve(t, []string{a.URL, b.URL}, http.MethodPost, "/api/v1/query", "query=up")

	require.Equal(t, 200, rec.Code)
	m := decode(t, rec)
	assert.Equal(t, "success", m.Status)
	assert.JSONEq(t, `{"resultType":"vector","result":[
		{"metric":{"cluster":"a"},"value":[1,"1"]},
		{"metric":{"cluster":"b"},"value":[1,"2"]}]}`, string(m.Data))
	assert.Empty(t, m.Warnings)
}

func TestMergeQueryRangeMatrix(t *testing.T) {
	t.Parallel()
	a := newUpstream(t, 200, success(`{"resultType":"matrix","result":[{"metric":{"cluster":"a"},"values":[[1,"1"]]}]}`))
	b := newUpstream(t, 200, success(`{"resultType":"matrix","result":[]}`))

	rec := serve(t, []string{a.URL, b.URL}, http.MethodPost, "/api/v1/query_range", "query=up&start=1&end=2&step=1")

	assert.JSONEq(t, `{"resultType":"matrix","result":[{"metric":{"cluster":"a"},"values":[[1,"1"]]}]}`, string(decode(t, rec).Data))
}

func TestMergeQueryScalarUsesFirstUpstream(t *testing.T) {
	t.Parallel()
	a := newUpstream(t, 200, success(`{"resultType":"scalar","result":[1,"1"]}`))
	b := newUpstream(t, 200, success(`{"resultType":"scalar","result":[1,"2"]}`))

	rec := serve(t, []string{a.URL, b.URL}, http.MethodPost, "/api/v1/query", "query=time()")

	assert.JSONEq(t, `{"resultType":"scalar","result":[1,"1"]}`, string(decode(t, rec).Data))
}

func TestMergeLabelsSeriesAndMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, path, a, b, expected string
	}{
		{"labels", "/api/v1/labels", `["job","cluster"]`, `["cluster","env"]`, `["cluster","env","job"]`},
		{"label values", "/api/v1/label/cluster/values", `["b","a"]`, `["c","a"]`, `["a","b","c"]`},
		{
			"series", "/api/v1/series",
			`[{"cluster":"b"},{"cluster":"a"}]`, `[{"cluster":"a"},{"cluster":"c"}]`,
			`[{"cluster":"a"},{"cluster":"b"},{"cluster":"c"}]`,
		},
		{
			"metadata", "/api/v1/metadata",
			`{"up":[{"type":"gauge","help":"h","unit":""}]}`,
			`{"up":[{"type":"gauge","help":"h","unit":""}],"x":[{"type":"counter","help":"x","unit":""}]}`,
			`{"up":[{"type":"gauge","help":"h","unit":""}],"x":[{"type":"counter","help":"x","unit":""}]}`,
		},
		{"empty", "/api/v1/labels", `[]`, `[]`, `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, b := newUpstream(t, 200, success(tt.a)), newUpstream(t, 200, success(tt.b))
			rec := serve(t, []string{a.URL, b.URL}, http.MethodGet, tt.path, "")
			require.Equal(t, 200, rec.Code)
			assert.JSONEq(t, tt.expected, string(decode(t, rec).Data))
		})
	}
}

func TestMergeFailedUpstreamBecomesWarning(t *testing.T) {
	t.Parallel()
	good := newUpstream(t, 200, success(`["a"]`))
	forbidden := newUpstream(t, 403, `{"status":"error","error":"no access"}`)
	garbage := newUpstream(t, 200, `not json`)
	unreachable := unreachableURL()

	rec := serve(t, []string{forbidden.URL, garbage.URL, unreachable, good.URL}, http.MethodGet, "/api/v1/labels", "")

	require.Equal(t, 200, rec.Code)
	m := decode(t, rec)
	assert.JSONEq(t, `["a"]`, string(m.Data))
	require.Len(t, m.Warnings, 3)
	assert.Contains(t, m.Warnings[0], forbidden.URL)
	assert.Contains(t, m.Warnings[0], "status 403")
	assert.Contains(t, m.Warnings[1], "invalid JSON")
	assert.Contains(t, m.Warnings[2], unreachable)
}

func TestMergeKeepsWarningsOfSuccessfulUpstreams(t *testing.T) {
	t.Parallel()
	partial := newUpstream(t, 200, `{"status":"success","data":["a"],"warnings":["partial results"]}`)
	failed := newUpstream(t, 503, `{"status":"error"}`)
	good := newUpstream(t, 200, success(`["b"]`))

	rec := serve(t, []string{partial.URL, failed.URL, good.URL}, http.MethodGet, "/api/v1/labels", "")

	require.Equal(t, 200, rec.Code)
	m := decode(t, rec)
	assert.JSONEq(t, `["a","b"]`, string(m.Data))
	// failed upstreams first, then the warnings of the upstreams which answered
	require.Len(t, m.Warnings, 2)
	assert.Contains(t, m.Warnings[0], failed.URL)
	assert.Equal(t, "upstream "+partial.URL+": partial results", m.Warnings[1])
}

func TestMergeSlowUpstreamTimesOutAndBecomesWarning(t *testing.T) {
	t.Parallel()
	slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // until the proxy gives up
	}))
	t.Cleanup(slow.Close)
	good := newUpstream(t, 200, success(`["a"]`))
	conf := &config.Config{
		PrometheusUrls:  []string{slow.URL, good.URL},
		Client:          &testAuthClient{},
		UpstreamTimeout: 250 * time.Millisecond,
	}

	rec := serveWith(t, conf, http.MethodGet, "/api/v1/labels", "")

	require.Equal(t, 200, rec.Code)
	m := decode(t, rec)
	assert.JSONEq(t, `["a"]`, string(m.Data))
	require.Len(t, m.Warnings, 1)
	assert.Contains(t, m.Warnings[0], slow.URL)
	assert.Contains(t, m.Warnings[0], "no response within 250ms")
}

func TestMergeAllFailed(t *testing.T) {
	t.Parallel()

	t.Run("returns the upstream error as is", func(t *testing.T) {
		t.Parallel()
		body := `{"status":"error","errorType":"bad_data","error":"parse error"}`
		a, b := newUpstream(t, 400, body), newUpstream(t, 503, `{"status":"error"}`)
		rec := serve(t, []string{a.URL, b.URL}, http.MethodPost, "/api/v1/query", "query=bad((")
		assert.Equal(t, 400, rec.Code)
		assert.Equal(t, body, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	})

	t.Run("bad gateway when nobody answered", func(t *testing.T) {
		t.Parallel()
		gone := unreachableURL()
		rec := serve(t, []string{gone, gone + "/x"}, http.MethodGet, "/api/v1/labels", "")
		assert.Equal(t, http.StatusBadGateway, rec.Code)
	})

	t.Run("bad gateway when the data cannot be merged", func(t *testing.T) {
		t.Parallel()
		a, b := newUpstream(t, 200, success(`"not a list"`)), newUpstream(t, 200, success(`"not a list"`))
		rec := serve(t, []string{a.URL, b.URL}, http.MethodGet, "/api/v1/labels", "")
		assert.Equal(t, http.StatusBadGateway, rec.Code)
		assert.Contains(t, rec.Body.String(), "failed to merge upstream responses")
	})

	t.Run("bad gateway when a 200 is not Prometheus", func(t *testing.T) {
		t.Parallel()
		a, b := newUpstream(t, 200, `<html>`), newUpstream(t, 200, `<html>`)
		rec := serve(t, []string{a.URL, b.URL}, http.MethodGet, "/api/v1/labels", "")
		assert.Equal(t, http.StatusBadGateway, rec.Code)
	})
}

func TestEveryUpstreamGetsTheRequestAndCredentials(t *testing.T) {
	t.Parallel()
	a, b := newUpstream(t, 200, success(`[]`)), newUpstream(t, 200, success(`[]`))

	serve(t, []string{a.URL, b.URL}, http.MethodPost, "/api/v1/series", "match[]=up")
	serve(t, []string{a.URL, b.URL}, http.MethodGet, "/api/v1/labels?limit=5", "")

	for _, f := range []*fakeUpstream{a, b} {
		requests, auth := f.received()
		assert.Equal(t, []string{
			"POST /api/v1/series match[]=up",
			"GET /api/v1/labels?limit=5 ",
		}, requests)
		assert.Equal(t, []string{"Bearer test-token", "Bearer test-token"}, auth)
	}
}

func TestPostWithQueryInURLAndEmptyBodyIsForwardedAsBodyToEveryUpstream(t *testing.T) {
	t.Parallel()
	a, b := newUpstream(t, 200, success(`[]`)), newUpstream(t, 200, success(`[]`))

	serve(t, []string{a.URL, b.URL}, http.MethodPost, "/api/v1/query?query=up", "")

	for _, f := range []*fakeUpstream{a, b} {
		requests, _ := f.received()
		assert.Equal(t, []string{"POST /api/v1/query query=up"}, requests)
	}
}

func TestEndpointsWhichAreNotMergedOnlyUseTheFirstUpstream(t *testing.T) {
	t.Parallel()
	a, b := newUpstream(t, 200, success(`{"activeTargets":[]}`)), newUpstream(t, 200, success(`{}`))

	rec := serve(t, []string{a.URL, b.URL}, http.MethodGet, "/api/v1/format_query?query=up", "")

	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, success(`{"activeTargets":[]}`), rec.Body.String(), "response is passed through unchanged")
	aRequests, _ := a.received()
	bRequests, _ := b.received()
	assert.Len(t, aRequests, 1)
	assert.Empty(t, bRequests)
}

// A single upstream, as every existing deployment is configured, behaves exactly as it
// did before merging existed: the response is returned as is, whatever it is
func TestSingleUpstreamPassesTheResponseThrough(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		path   string
		status int
		body   string
	}{
		{"query", "/api/v1/query", 200, success(`{"resultType":"vector","result":[],"stats":{"x":1}}`)},
		{"labels with warnings", "/api/v1/labels", 200, `{"status":"success","data":["b","a","a"],"warnings":["w"]}`},
		{"error status", "/api/v1/query", 422, `{"status":"error","errorType":"execution","error":"boom"}`},
		{"not prometheus", "/api/v1/labels", 200, `<html>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			up := newUpstream(t, tt.status, tt.body)
			rec := serve(t, []string{up.URL}, http.MethodGet, tt.path, "")
			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, tt.body, rec.Body.String())
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		})
	}

	t.Run("unreachable is a bad gateway", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, []string{unreachableURL()}, http.MethodGet, "/api/v1/labels", "")
		assert.Equal(t, http.StatusBadGateway, rec.Code)
		assert.Contains(t, rec.Body.String(), "failed to call upstream")
	})
}

func TestHandlerErrors(t *testing.T) {
	t.Parallel()

	t.Run("no urls", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, nil, http.MethodGet, "/api/v1/labels", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("headers error", func(t *testing.T) {
		t.Parallel()
		up := newUpstream(t, 200, success(`[]`))
		conf := &config.Config{PrometheusUrls: []string{up.URL, up.URL + "/x"}, Client: &testAuthClient{headersError: assert.AnError}}
		rec := serveWith(t, conf, http.MethodGet, "/api/v1/labels", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		requests, _ := up.received()
		assert.Empty(t, requests)
	})
}
