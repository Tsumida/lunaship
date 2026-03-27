package infra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Description:
// Start and stop dynamic pprof, then dump cpu/heap/goroutine profiles to local storage.
//
// Expectation:
// Stop writes three profile files with the expected suffixes into the configured dump directory.
func TestDynamicPprof_StartStop_DumpsProfiles(t *testing.T) {
	t.Setenv("APP_NAME", "unit-app")
	dumpDir := t.TempDir()
	manager := InitPprofServer("127.0.0.1:0", dumpDir)

	changed, err := manager.Start()
	assert.NoError(t, err, "starting dynamic pprof should succeed")
	assert.True(t, changed, "first start should transition from stopped to running")
	assert.Equal(t, "running", manager.State(), "manager state should be running after start")

	for range 100000 {
		_ = strings.Repeat("cpu", 2)
	}

	result, err := manager.Stop()
	assert.NoError(t, err, "stopping dynamic pprof should dump profiles cleanly")
	assert.True(t, result.Changed, "stop should transition from running to stopped")
	assert.Equal(t, "stopped", result.State, "manager state should report stopped after stop")
	assert.Len(t, result.DumpedFiles, 3, "stop should emit cpu, heap and goroutine profile files")
	assert.Equal(t, "stopped", manager.State(), "manager state should remain stopped after stop")

	assertProfileFile(t, result.DumpedFiles, pprofProfileCPU)
	assertProfileFile(t, result.DumpedFiles, pprofProfileHeap)
	assertProfileFile(t, result.DumpedFiles, pprofProfileGoroutine)

	entries, err := os.ReadDir(dumpDir)
	assert.NoError(t, err, "dump directory should be readable")
	assert.Len(t, entries, 3, "dump directory should contain exactly three profile files")
}

func assertProfileFile(t *testing.T, paths []string, profile string) {
	t.Helper()
	for _, path := range paths {
		if strings.HasSuffix(path, "-"+profile+".prof") {
			assert.Contains(t, filepath.Base(path), "pprof-unit-app-", "profile name should include APP_NAME")
			info, err := os.Stat(path)
			assert.NoError(t, err, "profile file should exist on disk")
			assert.Greater(t, info.Size(), int64(0), "profile file should not be empty")
			return
		}
	}
	assert.Failf(t, "missing profile", "expected %s profile in %v", profile, paths)
}
