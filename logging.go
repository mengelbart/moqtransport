package moqtransport

import (
	"log/slog"
	"math"
	"os"
	"strings"
)

const logEnv = "MOQ_LOG_LEVEL"

const (
	LogLevelNone = math.MaxInt
)

var moqtransportLogLevel = new(slog.LevelVar)

var defaultLogger *slog.Logger

func init() {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     moqtransportLogLevel,
	})
	moqtransportLogLevel.Set(readLoggingEnv())
	defaultLogger = slog.New(h)
}

func readLoggingEnv() slog.Level {
	switch strings.ToLower(os.Getenv(logEnv)) {
	case "":
		return LogLevelNone
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return LogLevelNone
	}
}
