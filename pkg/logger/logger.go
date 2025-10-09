package logger

import (
	"log/slog"

	"os"

	"sync"
)

var (
	loggerOnce sync.Once

	loggerVar *slog.Logger
)

func NewJSONLogger() *slog.Logger {
	loggerOnce.Do(func() {
		h := ContextHandler{Handler: slog.NewJSONHandler(os.Stdout, nil)}
		loggerVar = slog.New(h)
	})

	return loggerVar
}
