package infra

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap"
)

var (
	PROMETHEUS_LISTEN_ADDR = utils.StrOrDefault(os.Getenv("PROMETHEUS_LISTEN_ADDR"), ":9090")
)

// Dep: Log
func InitMetric(
	promListenAddr string,
) {
	GlobalLog().Info("starting prometheus", zap.String("addr", promListenAddr))
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(promListenAddr, nil); err != nil {
		GlobalLog().Error("failed to start prometheus", zap.Error(err))
	}
}
