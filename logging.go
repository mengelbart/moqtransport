package moqtransport

import (
	"log/slog"
)

const componentKey = "component"

func init() {
	defaultLogger = slog.Default()
}

var defaultLogger *slog.Logger

func SetLogHandler(handler slog.Handler) {
	defaultLogger = slog.New(handler)
}
