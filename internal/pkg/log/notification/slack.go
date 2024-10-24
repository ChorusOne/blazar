package notification

import (
	"fmt"

	"blazar/internal/pkg/config"

	"github.com/slack-go/slack"
)

type SlackNotifier struct {
	composeFile string
	hostname    string

	// webhook client
	webhookURL string

	// bot client
	client        *slack.Client
	channel       string
	groupMessages bool
}

func NewSlackNotifierFromConfig(cfg *config.Config, hostname string) *SlackNotifier {
	if cfg.Slack.BotNotifier != nil {
		return NewSlackBotNotifier(
			cfg.Slack.BotNotifier.AuthToken,
			cfg.Slack.BotNotifier.Channel,
			cfg.ComposeFile,
			hostname,
			cfg.Slack.BotNotifier.GroupMessages,
		)
	}
	return NewSlackWebhookNotifier(cfg.Slack.WebhookNotifier.WebhookURL, cfg.ComposeFile, hostname)
}

func NewSlackWebhookNotifier(webhookURL, composeFile, hostname string) *SlackNotifier {
	return &SlackNotifier{
		client:      nil,
		webhookURL:  webhookURL,
		hostname:    hostname,
		composeFile: composeFile,
		// using thread messages is only avaialable with slack bot client
		// because the webhook doesn't return the thread_ts of the message
		groupMessages: false,
	}
}

func NewSlackBotNotifier(token, channel, composeFile, hostname string, groupMessages bool) *SlackNotifier {
	return &SlackNotifier{
		client:        slack.New(token),
		webhookURL:    "",
		hostname:      hostname,
		composeFile:   composeFile,
		channel:       channel,
		groupMessages: groupMessages,
	}
}

func (s *SlackNotifier) NotifyInfo(message string, opts ...MsgOption) (string, error) {
	msg := "ℹ️ " + message
	return s.send(msg, opts)
}

func (s *SlackNotifier) NotifyWarn(message string, opts ...MsgOption) (string, error) {
	msg := fmt.Sprintf("⚠️ %s", message)
	return s.send(msg, opts)
}

func (s *SlackNotifier) NotifyErr(message string, opts ...MsgOption) (string, error) {
	msg := fmt.Sprintf("🚨 %s", message)
	return s.send(msg, opts)
}

func (s *SlackNotifier) send(message string, opts []MsgOption) (string, error) {
	cfg := optsToConfig(opts)

	contextBlock := slack.NewContextBlock(
		"context",
		slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("hostname: %s", s.hostname), true, false),
		slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("compose file: %s", s.composeFile), true, false),
	)

	fallbackMsg := fmt.Sprintf("%s\nhostname: %s\tcompose file: %s", message, s.hostname, s.composeFile)
	if cfg.err != nil {
		fallbackMsg = fmt.Sprintf("%s\nError: %s", message, cfg.err.Error())
		contextBlock.ContextElements.Elements = append([]slack.MixedElement{
			slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("error: %s", cfg.err.Error()), true, false),
		}, contextBlock.ContextElements.Elements...)
	}

	payload := slack.Attachment{
		Text: fallbackMsg,
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.SectionBlock{
					Type: "section",
					Text: &slack.TextBlockObject{
						Type:  "plain_text",
						Text:  message,
						Emoji: true,
					},
				},
				contextBlock,
			},
		},
	}

	if s.client != nil {
		var options []slack.MsgOption
		if s.groupMessages && cfg.parent != "" {
			options = append(options, slack.MsgOptionTS(cfg.parent))
		}

		options = append(options, slack.MsgOptionBlocks(payload.Blocks.BlockSet...))

		_, timestamp, err := s.client.PostMessage(s.channel, options...)
		return timestamp, err
	}

	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{payload},
	}
	return "", slack.PostWebhook(s.webhookURL, &msg)
}
