package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

var errForbiddenMethod = errors.New("forbidden request method")

type mockStatusResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type buildInfoData struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	Branch    string `json:"branch"`
	BuildUser string `json:"buildUser"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
}

type runtimeInfoData struct {
	StartTime           string `json:"startTime"`
	CWD                 string `json:"CWD"`
	ReloadConfigSuccess bool   `json:"reloadConfigSuccess"`
	LastConfigTime      string `json:"lastConfigTime"`
	TimeSeriesCount     int64  `json:"timeSeriesCount"`
	CorruptionCount     int64  `json:"corruptionCount"`
	GoroutineCount      int    `json:"goroutineCount"`
	GOMAXPROCS          int    `json:"GOMAXPROCS"`
	GOGC                string `json:"GOGC"`
	GODEBUG             string `json:"GODEBUG"`
	StorageRetention    string `json:"storageRetention"`
}

func (r *runtimeInfoData) update() {
	r.LastConfigTime = time.Now().UTC().Format(time.RFC3339Nano)
	r.GoroutineCount = runtime.NumGoroutine()
}

func NewBuildInfoData() *buildInfoData {
	return &buildInfoData{
		// https://github.com/prometheus/prometheus/releases/tag/v3.4.1
		Version:   "3.4.1",
		Revision:  "aea6503d9bbaad6c5faff3ecf6f1025213356c92",
		Branch:    "main",
		BuildUser: "prombot@github",
		GoVersion: runtime.Version(),
		BuildDate: time.Now().UTC().Format("20060102-15:04:05"),
	}
}

func NewRuntimeInfoData() *runtimeInfoData {
	return &runtimeInfoData{
		StartTime:           time.Now().UTC().Format(time.RFC3339Nano),
		CWD:                 "/",
		ReloadConfigSuccess: true,
		TimeSeriesCount:     0,
		CorruptionCount:     0,
		GoroutineCount:      runtime.NumGoroutine(),
		GOMAXPROCS:          runtime.GOMAXPROCS(0), // Passing 0 gets current value
		GOGC:                os.Getenv("GOGC"),
		GODEBUG:             os.Getenv("GODEBUG"),
		StorageRetention:    "30d", // Kiali doesn't like an empty string here
	}
}

func newConfigResponse() *mockStatusResponse {
	return &mockStatusResponse{
		Status: "success",
		Data: map[string]string{
			"yaml": "global:\n  scrape_interval: 15s\n",
		},
	}
}

func newBuildInfoResponse(buildInfo *buildInfoData) *mockStatusResponse {
	return &mockStatusResponse{
		Status: "success",
		Data:   buildInfo,
	}
}

func newRuntimeInfoResponse(runtimeInfo *runtimeInfoData) *mockStatusResponse {
	runtimeInfo.update()
	return &mockStatusResponse{
		Status: "success",
		Data:   runtimeInfo,
	}
}

// Implements a mock status endpoint that returns the configuration status of the proxy
func MockStatusConfigHandler(logger *logger.Logger) {
	http.HandleFunc("/api/v1/status/config", func(w http.ResponseWriter, r *http.Request) {
		l := logger.WithRequestFields(r)
		l.Info("processing request")

		if r.Method != http.MethodGet {
			l.Error("invalid request method")
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		response := newConfigResponse()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			l.Error("failed to encode config response", "error", err)
		}

		l.Info("request completed", "status_code", http.StatusOK)
	})
}

// Implements a mock status endpoint that returns dummy Prometheus runtime information
func MockStatusRuntimeInfoHandler(logger *logger.Logger, runtimeInfo *runtimeInfoData) {
	http.HandleFunc("/api/v1/status/runtimeinfo", func(w http.ResponseWriter, r *http.Request) {
		l := logger.WithRequestFields(r)
		l.Info("processing request")

		if r.Method != http.MethodGet {
			l.Error("invalid request method")
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		response := newRuntimeInfoResponse(runtimeInfo)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			l.Error("failed to encode runtime info response", "error", err)
		}

		l.Info("request completed", "status_code", http.StatusOK)
	})
}

// Implements a mock status endpoint that returns dummy Prometheus buildtime information
func MockStatusBuildInfoHandler(logger *logger.Logger, buildInfo *buildInfoData) {
	http.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, r *http.Request) {
		l := logger.WithRequestFields(r)
		l.Info("processing request")

		if r.Method != http.MethodGet {
			l.Error("invalid request method")
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		response := newBuildInfoResponse(buildInfo)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			l.Error("failed to encode runtime info response", "error", err)
		}

		l.Info("request completed", "status_code", http.StatusOK)
	})
}
