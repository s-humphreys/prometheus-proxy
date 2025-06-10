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

type RuntimeInfoData struct {
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

type RuntimeInfoResponse struct {
	Status string           `json:"status"`
	Data   *RuntimeInfoData `json:"data"`
}

func (r *RuntimeInfoData) update() {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	r.StartTime = ts
	r.LastConfigTime = ts
}

func newRuntimeInfoData() *RuntimeInfoData {
	return &RuntimeInfoData{
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

func newRuntimeInfoResponse(runtimeInfo *RuntimeInfoData) *RuntimeInfoResponse {
	runtimeInfo.update()
	return &RuntimeInfoResponse{
		Status: "success",
		Data:   runtimeInfo,
	}
}

// Implements a status endpoint that returns the configuration status of the proxy
func statusConfigRequestHandler(logger *logger.Logger) {
	http.HandleFunc("/api/v1/status/config", func(w http.ResponseWriter, r *http.Request) {
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data": map[string]string{
				"yaml": "global:\n  scrape_interval: 15s\n",
			},
		})

		cLog.Info("request completed", "status_code", http.StatusOK)
	})
}

// Implements a status endpoint that returns dummy Prometheus runtime information
func statusBuildRequestHandler(logger *logger.Logger, runtimeInfo *RuntimeInfoData) {
	http.HandleFunc("/api/v1/status/runtimeinfo", func(w http.ResponseWriter, r *http.Request) {
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

		response := newRuntimeInfoResponse(runtimeInfo)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			cLog.Error("failed to encode runtime info response", "error", err)
		}

		cLog.Info("request completed", "status_code", http.StatusOK)
	})
}
