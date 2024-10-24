package notification

import (
	"context"
	"sync"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/metrics"

	"github.com/rs/zerolog"
)

type FallbackNotifier struct {
	metrics  *metrics.Metrics
	logger   *zerolog.Logger
	notifier Notifier

	// map the first message of the thread to the upgrade height
	// and group the mssages into threaded conversation
	// if the underlying notifier supports it
	// TODO: the thread mapping is not persisted, so it will be lost on restart
	lock           sync.RWMutex
	upgradeThreads map[int64]string
}

// NewFallbackNotifier creates a new notifier with fallback to logger
func NewFallbackNotifier(cfg *config.Config, metrics *metrics.Metrics, logger *zerolog.Logger, hostname string) *FallbackNotifier {
	return &FallbackNotifier{
		metrics:  metrics,
		logger:   logger,
		notifier: NewNotifier(cfg, hostname),

		lock:           sync.RWMutex{},
		upgradeThreads: make(map[int64]string),
	}
}

func (cn *FallbackNotifier) NotifyInfo(ctx context.Context, message string) {
	if cn.notifier != nil {
		parentMessageID, upgradeHeight := cn.getParentMessage(ctx)
		messageID, err := cn.notifier.NotifyInfo(message, MsgOptionParent(parentMessageID))
		if err != nil {
			if cn.metrics != nil {
				cn.metrics.NotifErrs.Inc()
			}
			cn.logger.Error().Err(err).Msg("Failed to notify")
		} else {
			cn.registerUpgradeThread(upgradeHeight, parentMessageID, messageID)
		}
	}
}

func (cn *FallbackNotifier) NotifyWarnWithErr(ctx context.Context, message string, err error) {
	if cn.notifier != nil {
		parentMessageID, upgradeHeight := cn.getParentMessage(ctx)
		messageID, err := cn.notifier.NotifyWarn(message, MsgOptionParent(parentMessageID), MsgOptionError(err))
		if err != nil {
			if cn.metrics != nil {
				cn.metrics.NotifErrs.Inc()
			}
			cn.logger.Error().Err(err).Msg("Failed to notify")
		} else {
			cn.registerUpgradeThread(upgradeHeight, parentMessageID, messageID)
		}
	}
}

func (cn *FallbackNotifier) NotifyWarn(ctx context.Context, message string) {
	if cn.notifier != nil {
		parentMessageID, upgradeHeight := cn.getParentMessage(ctx)
		messageID, err := cn.notifier.NotifyWarn(message, MsgOptionParent(parentMessageID))
		if err != nil {
			if cn.metrics != nil {
				cn.metrics.NotifErrs.Inc()
			}
			cn.logger.Error().Err(err).Msg("Failed to notify")
		} else {
			cn.registerUpgradeThread(upgradeHeight, parentMessageID, messageID)
		}
	}
}

func (cn *FallbackNotifier) NotifyErr(ctx context.Context, message string, err error) {
	if cn.notifier != nil {
		parentMessageID, upgradeHeight := cn.getParentMessage(ctx)
		messageID, err := cn.notifier.NotifyErr(message, MsgOptionParent(parentMessageID), MsgOptionError(err))
		if err != nil {
			if cn.metrics != nil {
				cn.metrics.NotifErrs.Inc()
			}
			cn.logger.Error().Err(err).Msg("Failed to notify")
		} else {
			cn.registerUpgradeThread(upgradeHeight, parentMessageID, messageID)
		}
	}
}

func (cn *FallbackNotifier) registerUpgradeThread(upgradeHeight int64, parentMessageID, messageID string) {
	if upgradeHeight != 0 && parentMessageID == "" {
		cn.lock.Lock()
		defer cn.lock.Unlock()

		cn.upgradeThreads[upgradeHeight] = messageID
	}
}

func (cn *FallbackNotifier) getParentMessage(ctx context.Context) (parentMessageID string, upgradeHeight int64) {
	cn.lock.RLock()
	defer cn.lock.RUnlock()

	// check if upgrade height is set
	if upgradeHeight := ctx.Value(upgradeHeightKey{}); upgradeHeight != nil {
		if parentMessageID, ok := cn.upgradeThreads[upgradeHeight.(int64)]; ok {
			return parentMessageID, upgradeHeight.(int64)
		}
		return "", upgradeHeight.(int64)
	}

	return "", 0
}

type fallbackNotifierKey struct{}
type upgradeHeightKey struct{}

// WithUpgradeHeight returns a new context with the upgrade height
func WithUpgradeHeight(ctx context.Context, height int64) context.Context {
	return context.WithValue(ctx, upgradeHeightKey{}, height)
}

// WithContext returns a new context with the fallback notifier
func WithContextFallback(ctx context.Context, notifier *FallbackNotifier) context.Context {
	return context.WithValue(ctx, fallbackNotifierKey{}, notifier)
}

// FromContext returns fallback notifier from context or nil
func FromContextFallback(ctx context.Context) *FallbackNotifier {
	notifier := ctx.Value(fallbackNotifierKey{})
	if l, ok := notifier.(*FallbackNotifier); ok {
		return l
	}
	return nil
}
