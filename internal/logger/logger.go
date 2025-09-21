package logger

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"
)

var (
	LogLevelMap = map[string]slog.Level{
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
	if _, ok := LogLevelMap[logLevel]; !ok {
		return nil, fmt.Errorf("invalid log level: %s", logLevel)
	}

	opts := &slog.HandlerOptions{
		Level: LogLevelMap[logLevel],
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{slog.New(handler)}, nil
}

// WithRequestFields creates a new logger instance with a unique request ID
// and consistent fields useful to trace requests through the system
func (l *Logger) WithRequestFields(r *http.Request, fields ...any) *Logger {
	fields = append(fields,
		"request_id", uuid.New().String(),
		"method", r.Method,
		"url", r.URL.String(),
		"remote_addr", r.RemoteAddr,
	)

	return &Logger{Logger: l.Logger.With(fields...)}
}
