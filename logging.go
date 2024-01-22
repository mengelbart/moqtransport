package moqtransport

import (
	"log/slog"
	"os"
)

const componentKey = "component"

func init() {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     nil,
	})
	defaultLogger = slog.New(h)
}

var defaultLogger *slog.Logger

func SetLogHandler(handler slog.Handler) {
	defaultLogger = slog.New(handler)
}
