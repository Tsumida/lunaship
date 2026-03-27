package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	netpprof "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	runtimepprof "runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

var DEFAULT_PPROF_ADDR = ":6060"

const (
	defaultPprofDumpDir      = "/tmp"
	defaultPprofShutdownWait = 5 * time.Second
	pprofFilenameTimeLayout  = "20060102-150405"
	pprofProfileCPU          = "cpu"
	pprofProfileHeap         = "heap"
	pprofProfileGoroutine    = "goroutine"
)

type pprofState int32

const (
	pprofStateStopped pprofState = iota
	pprofStateRunning
	pprofStateDumping
)

type DynamicPprof struct {
	addr    string
	appName string
	dumpDir string

	state atomic.Int32

	mu        sync.Mutex
	server    *http.Server
	startedAt time.Time
	cpuFile   *os.File
	cpuTmp    string
}

type PprofStopResult struct {
	Changed     bool
	State       string
	DumpedFiles []string
}

type pprofControlResponse struct {
	State       string   `json:"state"`
	Changed     bool     `json:"changed"`
	Addr        string   `json:"addr,omitempty"`
	DumpedFiles []string `json:"dumped_files,omitempty"`
	Error       string   `json:"error,omitempty"`
}

var pprofDumpFailuresTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "lunaship_pprof_dump_failures_total",
		Help: "Total number of pprof dump failures by profile type.",
	},
	[]string{"profile"},
)

var pprofInitOnce sync.Once

func InitPprofServer(addr string, dumpDir string) *DynamicPprof {
	pprofInitOnce.Do(func() {
		registerPprofCollector(pprofDumpFailuresTotal)
	})
	return NewDynamicPprof(addr, dumpDir)
}

func NewDynamicPprof(addr string, dumpDir string) *DynamicPprof {
	manager := &DynamicPprof{
		addr:    utils.StrOrDefault(strings.TrimSpace(addr), DEFAULT_PPROF_ADDR),
		appName: strings.TrimSpace(os.Getenv("APP_NAME")),
		dumpDir: utils.StrOrDefault(strings.TrimSpace(dumpDir), defaultPprofDumpDir),
	}
	manager.state.Store(int32(pprofStateStopped))
	return manager
}

func (m *DynamicPprof) RegisterHandlers(mux *http.ServeMux) {
	if mux == nil {
		return
	}
	mux.HandleFunc("/pprof/start", m.handleStart)
	mux.HandleFunc("/pprof/stop", m.handleStop)
}

func (m *DynamicPprof) Start() (bool, error) {
	if !m.state.CompareAndSwap(int32(pprofStateStopped), int32(pprofStateRunning)) {
		return false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.dumpDir, 0o755); err != nil {
		m.state.Store(int32(pprofStateStopped))
		return false, err
	}

	startedAt := time.Now().UTC()
	cpuTmpPath := m.tmpCPUProfilePath(startedAt)
	cpuFile, err := os.Create(cpuTmpPath)
	if err != nil {
		m.state.Store(int32(pprofStateStopped))
		return false, err
	}

	if err := runtimepprof.StartCPUProfile(cpuFile); err != nil {
		_ = cpuFile.Close()
		_ = os.Remove(cpuTmpPath)
		m.state.Store(int32(pprofStateStopped))
		return false, err
	}

	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		runtimepprof.StopCPUProfile()
		_ = cpuFile.Close()
		_ = os.Remove(cpuTmpPath)
		m.state.Store(int32(pprofStateStopped))
		return false, err
	}

	server := &http.Server{
		Addr:    m.addr,
		Handler: newPprofServeMux(),
	}

	m.server = server
	m.startedAt = startedAt
	m.cpuFile = cpuFile
	m.cpuTmp = cpuTmpPath

	pprofLogger().Info(
		"pprof started",
		zap.String("addr", listener.Addr().String()),
		zap.String("app", m.appName),
		zap.String("cpu_profile_tmp", cpuTmpPath),
	)

	go utils.Go(func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			pprofLogger().Error("pprof server down", zap.Error(serveErr))
		}
	})

	return true, nil
}

func (m *DynamicPprof) Stop() (PprofStopResult, error) {
	if !m.state.CompareAndSwap(int32(pprofStateRunning), int32(pprofStateDumping)) {
		return PprofStopResult{
			Changed: false,
			State:   m.State(),
		}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	result := PprofStopResult{
		Changed: true,
		State:   pprofStateStopped.String(),
	}

	startedAt := m.startedAt
	server := m.server
	cpuFile := m.cpuFile
	cpuTmp := m.cpuTmp

	m.server = nil
	m.cpuFile = nil
	m.cpuTmp = ""
	m.startedAt = time.Time{}

	var errs []error

	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), defaultPprofShutdownWait)
		if err := server.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		cancel()
	}

	if cpuFile != nil {
		runtimepprof.StopCPUProfile()
		if err := cpuFile.Close(); err != nil {
			pprofDumpFailuresTotal.WithLabelValues(pprofProfileCPU).Inc()
			errs = append(errs, fmt.Errorf("close cpu profile: %w", err))
		} else {
			finalCPUPath := m.profilePath(startedAt, pprofProfileCPU)
			if err := os.Rename(cpuTmp, finalCPUPath); err != nil {
				pprofDumpFailuresTotal.WithLabelValues(pprofProfileCPU).Inc()
				errs = append(errs, fmt.Errorf("rename cpu profile: %w", err))
			} else {
				result.DumpedFiles = append(result.DumpedFiles, finalCPUPath)
			}
		}
	}

	runtime.GC()

	heapPath, err := m.dumpNamedProfile(pprofProfileHeap, startedAt)
	if err != nil {
		errs = append(errs, err)
	} else {
		result.DumpedFiles = append(result.DumpedFiles, heapPath)
	}

	goroutinePath, err := m.dumpNamedProfile(pprofProfileGoroutine, startedAt)
	if err != nil {
		errs = append(errs, err)
	} else {
		result.DumpedFiles = append(result.DumpedFiles, goroutinePath)
	}

	m.state.Store(int32(pprofStateStopped))

	if len(errs) > 0 {
		err := errors.Join(errs...)
		pprofLogger().Error(
			"pprof stopped with dump errors",
			zap.Strings("dumped_files", result.DumpedFiles),
			zap.Error(err),
		)
		return result, err
	}

	pprofLogger().Info(
		"pprof stopped",
		zap.Strings("dumped_files", result.DumpedFiles),
	)
	return result, nil
}

func (m *DynamicPprof) State() string {
	return pprofState(m.state.Load()).String()
}

func (m *DynamicPprof) handleStart(w http.ResponseWriter, r *http.Request) {
	if !allowPprofControlMethod(r.Method) {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	changed, err := m.Start()
	resp := pprofControlResponse{
		State:   m.State(),
		Changed: changed,
		Addr:    m.addr,
	}
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
		resp.Error = err.Error()
	}
	writePprofResponse(w, status, resp)
}

func (m *DynamicPprof) handleStop(w http.ResponseWriter, r *http.Request) {
	if !allowPprofControlMethod(r.Method) {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := m.Stop()
	resp := pprofControlResponse{
		State:       result.State,
		Changed:     result.Changed,
		DumpedFiles: result.DumpedFiles,
	}
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
		resp.Error = err.Error()
	}
	writePprofResponse(w, status, resp)
}

func (m *DynamicPprof) dumpNamedProfile(name string, startedAt time.Time) (string, error) {
	profile := runtimepprof.Lookup(name)
	if profile == nil {
		pprofDumpFailuresTotal.WithLabelValues(name).Inc()
		return "", fmt.Errorf("%s profile not found", name)
	}

	path := m.profilePath(startedAt, name)
	file, err := os.Create(path)
	if err != nil {
		pprofDumpFailuresTotal.WithLabelValues(name).Inc()
		return "", err
	}
	defer file.Close()

	if err := profile.WriteTo(file, 0); err != nil {
		pprofDumpFailuresTotal.WithLabelValues(name).Inc()
		_ = os.Remove(path)
		return "", err
	}

	return path, nil
}

func (m *DynamicPprof) profilePath(startedAt time.Time, profile string) string {
	filename := fmt.Sprintf(
		"pprof-%s-%s-%s.prof",
		m.appName,
		startedAt.UTC().Format(pprofFilenameTimeLayout),
		profile,
	)
	return filepath.Join(m.dumpDir, filename)
}

func (m *DynamicPprof) tmpCPUProfilePath(startedAt time.Time) string {
	filename := fmt.Sprintf(
		"pprof-%s-%s-%s.tmp",
		m.appName,
		startedAt.UTC().Format(pprofFilenameTimeLayout),
		pprofProfileCPU,
	)
	return filepath.Join(m.dumpDir, filename)
}

func writePprofResponse(w http.ResponseWriter, status int, resp pprofControlResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func allowPprofControlMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodPost
}

func pprofLogger() *zap.Logger {
	logger := log.GlobalLog()
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

func registerPprofCollector(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		var alreadyRegisteredErr prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegisteredErr) {
			return
		}
		panic(err)
	}
}

func newPprofServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", netpprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", netpprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", netpprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", netpprof.Trace)
	return mux
}

func InitGopprof(addr string) error {
	go utils.Go(func() {
		if err := http.ListenAndServe(utils.StrOrDefault(strings.TrimSpace(addr), DEFAULT_PPROF_ADDR), newPprofServeMux()); err != nil {
			pprofLogger().Error("failed to start static pprof", zap.Error(err))
		}
	})
	return nil
}

func (s pprofState) String() string {
	switch s {
	case pprofStateRunning:
		return "running"
	case pprofStateDumping:
		return "dumping"
	default:
		return "stopped"
	}
}
