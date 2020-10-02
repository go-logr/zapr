package zapr

import (
	"testing"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func infoLogger(t *testing.T) logr.Logger {
	t.Helper()

	config := zap.NewProductionConfig()
	config.Level.SetLevel(zapcore.InfoLevel)
	zapLogger, err := config.Build()
	if err != nil {
		t.FailNow()
	}

	return NewLogger(zapLogger)
}

func TestZapLogger_Enabled_Info(t *testing.T) {
	zl := infoLogger(t)
	if !zl.V(0).Enabled() {
		t.Error("V(0) should be enabled at InfoLevel")
	}

	if zl.V(1).Enabled() {
		t.Error("V(1) should not be enabled at InfoLevel")
	}
}

func TestZapLogger_WithName(t *testing.T) {
	zl := infoLogger(t)
	if zl.V(1).WithName("logger").Enabled() {
		t.Error("V(1).WithName() should not be enabled at InfoLevel")
	}
}
