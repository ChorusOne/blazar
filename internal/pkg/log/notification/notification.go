package notification

import (
	"blazar/internal/pkg/config"
)

type Notifier interface {
	NotifyInfo(message string, opts ...MsgOption) (string, error)
	NotifyWarn(message string, opts ...MsgOption) (string, error)
	NotifyErr(message string, opts ...MsgOption) (string, error)
}

type notifierConfig struct {
	parent string
	err    error
}

type MsgOption func(*notifierConfig)

func MsgOptionParent(parentID string) MsgOption {
	return func(config *notifierConfig) {
		config.parent = parentID
	}
}

func MsgOptionError(err error) MsgOption {
	return func(config *notifierConfig) {
		config.err = err
	}
}

func NewNotifier(cfg *config.Config, hostname string) Notifier {
	if cfg.Slack != nil {
		return NewSlackNotifierFromConfig(cfg, hostname)
	}
	return nil
}

func optsToConfig(opts []MsgOption) notifierConfig {
	var cfg notifierConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
