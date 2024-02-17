package infra

import (
	"os"
	"testing"

	"github.com/tsumida/lunaship/infra/utils"
	"go.uber.org/zap/zapcore"
)

func InteragationTest(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("integration-disabled")
	}

	PrepareLog(t)
}

func PrepareLog(t *testing.T) {
	_ = InitLog(
		utils.StrOrDefault(os.Getenv("LOG_FILE"), "stdout"),
		utils.StrOrDefault(os.Getenv("ERR_FILE"), "stdout"),
		zapcore.InfoLevel,
	)
}
