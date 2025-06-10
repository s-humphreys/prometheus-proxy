package logger

import (
	"fmt"
	"log/slog"
	"os"
)

var (
	logLevelMap = map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}
)

type Logger struct {
	*slog.Logger
}

func New(logLevel string) (*Logger, error) {
	if _, ok := logLevelMap[logLevel]; !ok {
		return nil, fmt.Errorf("invalid log level: %s", logLevel)
	}

	opts := &slog.HandlerOptions{
		Level: logLevelMap[logLevel],
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{slog.New(handler)}, nil
}
