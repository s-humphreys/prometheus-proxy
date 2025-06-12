package proxy

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

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
	CorruptionCount     int64  `json:"corruptionCount"`
	GoroutineCount      int    `json:"goroutineCount"`
	GOMAXPROCS          int    `json:"GOMAXPROCS"`
	GOMEMLIMIT          int64  `json:"GOMEMLIMIT"`
	GOGC                string `json:"GOGC"`
	GODEBUG             string `json:"GODEBUG"`
	StorageRetention    string `json:"storageRetention"`
}

func (r *runtimeInfoData) update() {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	r.StartTime = ts
	r.LastConfigTime = ts
}

func newbuildInfoData() *buildInfoData {
	return &buildInfoData{
		Version:   "3.4.1",
		GoVersion: runtime.Version(),
		BuildDate: time.Now().UTC().Format("20060102-15:04:05"),
	}
}

func newruntimeInfoData() *runtimeInfoData {
	return &runtimeInfoData{
		CWD:                 "/",
		ReloadConfigSuccess: true,
		CorruptionCount:     0,
		GoroutineCount:      runtime.NumGoroutine(),
		GOMAXPROCS:          runtime.GOMAXPROCS(0), // Passing 0 gets current value
		GOMEMLIMIT:          9223372036854775807,   // Default Go value
		GOGC:                os.Getenv("GOGC"),
		GODEBUG:             os.Getenv("GODEBUG"),
		StorageRetention:    "360h", // Kiali doesn't like an empty string here
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
func mockStatusConfigHandler(logger *logger.Logger) {
	http.HandleFunc("/api/v1/status/config", func(w http.ResponseWriter, r *http.Request) {
		requestId := uuid.New().String()
		l := logger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestId,
			"remote_addr", r.RemoteAddr,
		)

		l.Info("processing request")

		if r.Method != http.MethodGet {
			l.Error("invalid request method")
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data": map[string]string{
				"yaml": "global:\n  scrape_interval: 15s\n",
			},
		})

		l.Info("request completed", "status_code", http.StatusOK)
	})
}

// Implements a mock status endpoint that returns dummy Prometheus runtime information
func mockStatusRuntimeInfoHandler(logger *logger.Logger, runtimeInfo *runtimeInfoData) {
	http.HandleFunc("/api/v1/status/runtimeinfo", func(w http.ResponseWriter, r *http.Request) {
		requestId := uuid.New().String()
		l := logger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestId,
			"remote_addr", r.RemoteAddr,
		)

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
func mockStatusBuildInfoHandler(logger *logger.Logger, buildInfo *buildInfoData) {
	http.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, r *http.Request) {
		requestId := uuid.New().String()
		l := logger.With(
			"method", r.Method,
			"url", r.URL.String(),
			"request_id", requestId,
			"remote_addr", r.RemoteAddr,
		)

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
