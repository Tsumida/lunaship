package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsumida/lunaship/infra"
)

type pprofControlResponseBody struct {
	State       string   `json:"state"`
	Changed     bool     `json:"changed"`
	Addr        string   `json:"addr"`
	DumpedFiles []string `json:"dumped_files"`
	Error       string   `json:"error"`
}

// Description:
// Exercise `/pprof/start` and `/pprof/stop` handlers through the HTTP control mux.
//
// Expectation:
// Repeated start/stop requests are idempotent and return a stable state payload.
func TestDynamicPprof_Handlers_Idempotent(t *testing.T) {
	t.Setenv("APP_NAME", "http-service")
	dumpDir := t.TempDir()
	manager := infra.InitPprofServer("127.0.0.1:0", dumpDir)
	mux := http.NewServeMux()
	manager.RegisterHandlers(mux)

	server := httptest.NewServer(mux)
	defer server.Close()

	startResp := callPprofControlEndpoint(t, server.Client(), server.URL+"/pprof/start")
	assert.Equal(t, "running", startResp.State, "first start should report running")
	assert.True(t, startResp.Changed, "first start should change state")
	assert.NotEmpty(t, startResp.Addr, "start response should expose the pprof listen address")

	startResp = callPprofControlEndpoint(t, server.Client(), server.URL+"/pprof/start")
	assert.Equal(t, "running", startResp.State, "second start should keep running state")
	assert.False(t, startResp.Changed, "second start should be a no-op")

	stopResp := callPprofControlEndpoint(t, server.Client(), server.URL+"/pprof/stop")
	assert.Equal(t, "stopped", stopResp.State, "first stop should report stopped")
	assert.True(t, stopResp.Changed, "first stop should change state")
	assert.Len(t, stopResp.DumpedFiles, 3, "first stop should return dumped profile files")

	stopResp = callPprofControlEndpoint(t, server.Client(), server.URL+"/pprof/stop")
	assert.Equal(t, "stopped", stopResp.State, "second stop should keep stopped state")
	assert.False(t, stopResp.Changed, "second stop should be a no-op")
	assert.Empty(t, stopResp.DumpedFiles, "no-op stop should not return new dump files")
}

func callPprofControlEndpoint(t *testing.T, client *http.Client, url string) pprofControlResponseBody {
	t.Helper()
	resp, err := client.Post(url, "application/json", nil)
	assert.NoError(t, err, "pprof control request should succeed")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "pprof control endpoint should return 200")

	var body pprofControlResponseBody
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err, "pprof control response should be valid json")
	assert.Empty(t, body.Error, "pprof control response should not contain an error")
	return body
}
