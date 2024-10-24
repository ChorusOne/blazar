package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type loggerKey struct{}

func NewLogger() *zerolog.Logger {
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.Kitchen}
	logger := zerolog.New(output).With().Str("module", "blazar").Timestamp().Logger()
	return &logger
}

func SetGlobalLogLevel(level int8) {
	zerolog.SetGlobalLevel(zerolog.Level(level))
}

// WithContext returns a new context with the logger
func WithContext(ctx context.Context, logger *zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the logger in the context if it exists, otherwise a new logger is returned
func FromContext(ctx context.Context) *zerolog.Logger {
	logger := ctx.Value(loggerKey{})
	if l, ok := logger.(*zerolog.Logger); ok {
		return l
	}
	return NewLogger()
}
