package log

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"blazar/internal/pkg/log/logger"
	"blazar/internal/pkg/log/notification"
)

type MultiLogger struct {
	logger   *zerolog.Logger
	notifier *notification.FallbackNotifier

	level zerolog.Level
	msg   string
	err   error
}

func FromContext(ctx context.Context) *MultiLogger {
	return &MultiLogger{
		logger:   logger.FromContext(ctx),
		notifier: notification.FromContextFallback(ctx),
	}
}

func (c *MultiLogger) WithContext(ctx context.Context) context.Context {
	ctx = logger.WithContext(ctx, c.logger)
	ctx = notification.WithContextFallback(ctx, c.notifier)

	return ctx
}

func (c *MultiLogger) With(key, value string) *MultiLogger {
	newLogger := c.logger.With().Str(key, value).Logger()
	c.logger = &newLogger
	return c
}

func (c *MultiLogger) Debug(msg string) *MultiLogger {
	l := newLogger(c, msg, zerolog.DebugLevel)
	if c.err != nil {
		l.logger.Debug().Err(c.err).Msg(c.msg)
		return l.Err(c.err)
	}

	l.logger.Debug().Msg(msg)

	return l
}

func (c *MultiLogger) Debugf(format string, v ...interface{}) *MultiLogger {
	return c.Debug(fmt.Sprintf(format, v...))
}

func (c *MultiLogger) Info(msg string) *MultiLogger {
	l := newLogger(c, msg, zerolog.InfoLevel)
	if c.err != nil {
		l.logger.Info().Err(c.err).Msg(c.msg)
		return l.Err(c.err)
	}

	l.logger.Info().Msg(msg)

	return l
}

func (c *MultiLogger) Infof(format string, v ...interface{}) *MultiLogger {
	return c.Info(fmt.Sprintf(format, v...))
}

func (c *MultiLogger) Warn(msg string) *MultiLogger {
	l := newLogger(c, msg, zerolog.WarnLevel)
	if c.err != nil {
		l.logger.Warn().Err(c.err).Msg(c.msg)
		return l.Err(c.err)
	}

	l.logger.Warn().Msg(msg)

	return l
}

func (c *MultiLogger) Warnf(format string, v ...interface{}) *MultiLogger {
	return c.Warn(fmt.Sprintf(format, v...))
}

func (c *MultiLogger) Error(msg string) *MultiLogger {
	l := newLogger(c, msg, zerolog.ErrorLevel)
	l.logger.Error().Err(c.err).Msg(msg)

	return l.Err(c.err)
}

func (c *MultiLogger) Errorf(err error, format string, v ...interface{}) *MultiLogger {
	return c.Err(err).Error(fmt.Sprintf(format, v...))
}

func (c *MultiLogger) Notify(ctx context.Context, msgs ...string) *MultiLogger {
	msg := c.msg
	if len(msgs) > 0 {
		msg = strings.Join(msgs, " ")
	}

	switch c.level {
	case zerolog.InfoLevel:
		c.notifier.NotifyInfo(ctx, msg)
	case zerolog.WarnLevel:
		if c.err == nil {
			c.notifier.NotifyWarn(ctx, msg)
		} else {
			c.notifier.NotifyWarnWithErr(ctx, msg, c.err)
		}
	case zerolog.ErrorLevel:
		c.notifier.NotifyErr(ctx, msg, c.err)
	default:
		panic("unsupported log level for notification")
	}

	return c
}

func (c *MultiLogger) Notifyf(ctx context.Context, format string, v ...interface{}) *MultiLogger {
	return c.Notify(ctx, fmt.Sprintf(format, v...))
}

func (c *MultiLogger) Err(err error) *MultiLogger {
	return &MultiLogger{
		logger:   c.logger,
		notifier: c.notifier,
		level:    c.level,
		msg:      c.msg,
		err:      err,
	}
}

func newLogger(c *MultiLogger, msg string, level zerolog.Level) *MultiLogger {
	return &MultiLogger{
		logger:   c.logger,
		notifier: c.notifier,
		level:    level,
		msg:      msg,
	}
}
