package infra

import (
	"net/http"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tsumida/lunaship/log"
	"github.com/tsumida/lunaship/utils"
	"go.uber.org/zap"
)

var (
	PROMETHEUS_LISTEN_ADDR = utils.StrOrDefault(os.Getenv("PROMETHEUS_LISTEN_ADDR"), ":9090")
	metricHandlerOnce      sync.Once
)

// Dep: Log
func InitMetric(
	promListenAddr string,
) {
	log.GlobalLog().Info("starting prometheus", zap.String("addr", promListenAddr))
	metricHandlerOnce.Do(func() {
		http.Handle("/metrics", promhttp.Handler())
	})
	if err := http.ListenAndServe(promListenAddr, nil); err != nil {
		log.GlobalLog().Error("failed to start prometheus", zap.Error(err))
	}
}

// Metric reporter
// Dep: Log
func InitMetricAsync(promListenAddr string) {
	go utils.Go(func() {
		InitMetric(promListenAddr)
	})
}
