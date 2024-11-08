package moqtransport

import (
	"log/slog"
	"os"
)

func init() {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	})
	defaultLogger = slog.New(h)
}

var defaultLogger *slog.Logger

func SetLogHandler(handler slog.Handler) {
	defaultLogger = slog.New(handler)
}
