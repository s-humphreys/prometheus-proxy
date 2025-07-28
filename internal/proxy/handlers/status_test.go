package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/s-humphreys/prometheus-proxy/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuildInfoData(t *testing.T) {
	buildInfo := NewBuildInfoData()

	assert.Equal(t, "3.4.1", buildInfo.Version)
	assert.Equal(t, "aea6503d9bbaad6c5faff3ecf6f1025213356c92", buildInfo.Revision)
	assert.Equal(t, "main", buildInfo.Branch)
	assert.Equal(t, "prombot@github", buildInfo.BuildUser)
	assert.Equal(t, runtime.Version(), buildInfo.GoVersion)
	assert.NotEmpty(t, buildInfo.BuildDate)
}

func TestNewRuntimeInfoData(t *testing.T) {
	runtimeInfo := NewRuntimeInfoData()

	assert.NotEmpty(t, runtimeInfo.StartTime)
	assert.Equal(t, "/", runtimeInfo.CWD)
	assert.True(t, runtimeInfo.ReloadConfigSuccess)
	assert.Equal(t, int64(0), runtimeInfo.TimeSeriesCount)
	assert.Equal(t, int64(0), runtimeInfo.CorruptionCount)
	assert.Greater(t, runtimeInfo.GoroutineCount, 0)
	assert.Greater(t, runtimeInfo.GOMAXPROCS, 0)
	assert.Equal(t, "30d", runtimeInfo.StorageRetention)
}

func TestRuntimeInfoData_Update(t *testing.T) {
	runtimeInfo := NewRuntimeInfoData()
	originalLastConfigTime := runtimeInfo.LastConfigTime
	originalGoroutineCount := runtimeInfo.GoroutineCount

	// Wait a tiny bit to ensure time difference
	runtimeInfo.update()

	// LastConfigTime should be updated
	assert.NotEqual(t, originalLastConfigTime, runtimeInfo.LastConfigTime)
	// GoroutineCount might be different
	assert.GreaterOrEqual(t, runtimeInfo.GoroutineCount, originalGoroutineCount)
}

func TestMockStatusConfigHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "GET returns config",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:           "POST forbidden",
			method:         "POST",
			expectedStatus: http.StatusForbidden,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh mux for each test
			mux := http.NewServeMux()

			// Setup the handler inline to avoid global state
			mux.HandleFunc("/api/v1/status/config", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
					return
				}

				response := newConfigResponse()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			})

			// Create request and recorder
			req := testutil.CreateHTTPRequest(t, tt.method, "/api/v1/status/config", nil)
			recorder := httptest.NewRecorder()

			// Execute
			mux.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if !tt.expectedError {
				// Check JSON response structure
				var response mockStatusResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "success", response.Status)
				assert.NotNil(t, response.Data)
			}
		})
	}
}

func TestMockStatusBuildInfoHandler(t *testing.T) {
	buildInfo := NewBuildInfoData()

	// Create a fresh mux
	mux := http.NewServeMux()

	// Setup the handler inline
	mux.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		response := newBuildInfoResponse(buildInfo)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	// Create request and recorder
	req := testutil.CreateHTTPRequest(t, "GET", "/api/v1/status/buildinfo", nil)
	recorder := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(recorder, req)

	// Assert
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	// Check JSON response structure
	var response mockStatusResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)

	// Check build info data - response.Data is an interface{} that contains a map
	dataMap, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "3.4.1", dataMap["version"])
	assert.Equal(t, runtime.Version(), dataMap["goVersion"])
}

func TestMockStatusRuntimeInfoHandler(t *testing.T) {
	runtimeInfo := NewRuntimeInfoData()

	// Create a fresh mux
	mux := http.NewServeMux()

	// Setup the handler inline
	mux.HandleFunc("/api/v1/status/runtimeinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, errForbiddenMethod.Error(), http.StatusForbidden)
			return
		}

		response := newRuntimeInfoResponse(runtimeInfo)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	// Create request and recorder
	req := testutil.CreateHTTPRequest(t, "GET", "/api/v1/status/runtimeinfo", nil)
	recorder := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(recorder, req)

	// Assert
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	// Check JSON response structure
	var response mockStatusResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.NotNil(t, response.Data)
}
