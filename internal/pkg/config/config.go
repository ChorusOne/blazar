package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"blazar/internal/pkg/errors"
	checksproto "blazar/internal/pkg/proto/daemon"
	urproto "blazar/internal/pkg/proto/upgrades_registry"

	"github.com/BurntSushi/toml"
	"golang.org/x/sys/unix"
)

type UpgradeMode string

const (
	UpgradeInComposeFile UpgradeMode = "compose-file"
	UpgradeInEnvFile     UpgradeMode = "env-file"
)

var ValidUpgradeModes = []UpgradeMode{UpgradeInEnvFile, UpgradeInComposeFile}

type SlackWebhookNotifier struct {
	WebhookURL string `toml:"webhook-url"`
}

type SlackBotNotifier struct {
	AuthToken     string `toml:"auth-token"`
	Channel       string `toml:"channel"`
	GroupMessages bool   `toml:"group-messages"`
}

type Slack struct {
	WebhookNotifier *SlackWebhookNotifier `toml:"webhook-notifier"`
	BotNotifier     *SlackBotNotifier     `toml:"bot-notifier"`
}

type DockerCredentialHelper struct {
	Command string        `toml:"command"`
	Timeout time.Duration `toml:"timeout"`
}

type Watchers struct {
	UIInterval time.Duration `toml:"upgrade-info-interval"`
	HInterval  time.Duration `toml:"height-interval"`
	HTimeout   time.Duration `toml:"height-timeout"`
	UPInterval time.Duration `toml:"upgrade-proposals-interval"`
}

type Clients struct {
	Host         string        `toml:"host"`
	GrpcPort     uint16        `toml:"grpc-port"`
	CometbftPort uint16        `toml:"cometbft-port"`
	Timeout      time.Duration `toml:"timeout"`
}

type ComposeCli struct {
	DownTimeout time.Duration `toml:"down-timeout"`
	UpDeadline  time.Duration `toml:"up-deadline"`
	EnvPrefix   string        `toml:"env-prefix"`
}

type SslMode string

const (
	Disable    SslMode = "disable"
	Allow      SslMode = "allow"
	Prefer     SslMode = "prefer"
	Require    SslMode = "require"
	VerifyCa   SslMode = "verify-ca"
	VerifyFull SslMode = "verify-full"
)

type ChainProvider struct {
	DefaultPriority int32 `toml:"default-priority"`
}

type DatabaseProvider struct {
	DefaultPriority int32   `toml:"default-priority"`
	Host            string  `toml:"host"`
	Port            uint16  `toml:"port"`
	DB              string  `toml:"db"`
	User            string  `toml:"user"`
	Password        string  `toml:"password"`
	PasswordFile    string  `toml:"password-file"`
	SslMode         SslMode `toml:"ssl-mode"`
	AutoMigrate     bool    `toml:"auto-migrate"`
}

type LocalProvider struct {
	DefaultPriority int32  `toml:"default-priority"`
	ConfigPath      string `toml:"config-path"`
}

type Provider struct {
	Chain    *ChainProvider    `toml:"chain"`
	Database *DatabaseProvider `toml:"database"`
	Local    *LocalProvider    `toml:"local"`
}

type VersionResolvers struct {
	Providers []string `toml:"providers"`
}

type StateMachine struct {
	Provider string `toml:"provider"`
}

type PreUpgrade struct {
	Enabled       []string       `toml:"enabled"`
	Blocks        int64          `toml:"blocks"`
	SetHaltHeight *SetHaltHeight `toml:"set-halt-height"`
}

type SetHaltHeight struct {
	DelayBlocks int64 `toml:"delay-blocks"`
}

type GrpcResponsive struct {
	PollInterval time.Duration `toml:"poll-interval"`
	Timeout      time.Duration `toml:"timeout"`
}

type ChainHeightIncreased struct {
	PollInterval  time.Duration `toml:"poll-interval"`
	NotifInterval time.Duration `toml:"notif-interval"`
	Timeout       time.Duration `toml:"timeout"`
}

type FirstBlockVoted struct {
	PollInterval  time.Duration `toml:"poll-interval"`
	NotifInterval time.Duration `toml:"notif-interval"`
	Timeout       time.Duration `toml:"timeout"`
}

type PostUpgrade struct {
	Enabled              []string              `toml:"enabled"`
	GrpcResponsive       *GrpcResponsive       `toml:"grpc-responsive"`
	ChainHeightIncreased *ChainHeightIncreased `toml:"chain-height-increased"`
	FirstBlockVoted      *FirstBlockVoted      `toml:"first-block-voted"`
}

type Checks struct {
	PreUpgrade  PreUpgrade  `toml:"pre-upgrade"`
	PostUpgrade PostUpgrade `toml:"post-upgrade"`
}

type UpgradeRegistry struct {
	Network           string            `toml:"network"`
	Provider          Provider          `toml:"provider"`
	SelectedProviders []string          `toml:"providers"`
	VersionResolvers  *VersionResolvers `toml:"version-resolvers"`
	StateMachine      StateMachine      `toml:"state-machine"`
}

// The validation of the config and the order of prams in the sample
// toml files follow a DFS traversal of the struct.
type Config struct {
	ComposeFile      string                  `toml:"compose-file"`
	ComposeService   string                  `toml:"compose-service"`
	VersionFile      string                  `toml:"version-file"`
	UpgradeMode      UpgradeMode             `toml:"upgrade-mode"`
	ChainHome        string                  `toml:"chain-home"`
	LogLevel         int8                    `toml:"log-level"`
	Host             string                  `toml:"host"`
	GrpcPort         uint16                  `toml:"grpc-port"`
	HTTPPort         uint16                  `toml:"http-port"`
	ChainID          string                  `toml:"chain-id"`
	Watchers         Watchers                `toml:"watchers"`
	Clients          Clients                 `toml:"clients"`
	Compose          ComposeCli              `toml:"compose-cli"`
	Checks           Checks                  `toml:"checks"`
	Slack            *Slack                  `toml:"slack"`
	CredentialHelper *DockerCredentialHelper `toml:"docker-credential-helper"`
	UpgradeRegistry  UpgradeRegistry         `toml:"upgrade-registry"`
}

func ReadEnvVar(key string) string {
	return os.Getenv("BLAZAR_" + key)
}

func ReadConfig(cfgFile string) (*Config, error) {
	var config Config
	_, err := toml.DecodeFile(cfgFile, &config)
	if err != nil {
		return nil, errors.Wrapf(err, "could not decode config file")
	}
	return &config, nil
}

func (cfg *Config) UpgradeInfoFilePath() string {
	return filepath.Join(cfg.ChainHome, "data", "upgrade-info.json")
}

func checkAccess(path string, permBits uint32) error {
	err := unix.Access(path, permBits)
	if err != nil {
		return errors.Wrapf(err, "requested permission bits %03b not found on %q", permBits, path)
	}
	return nil
}

func validateDir(dir string, permBits uint32) error {
	switch {
	case !filepath.IsAbs(dir):
		return fmt.Errorf("%q must be an absolute path", dir)
	default:
		switch dirStat, err := os.Stat(dir); {
		case os.IsNotExist(err):
			return errors.Wrapf(err, "directory not found")
		case err != nil:
			return errors.Wrapf(err, "could not stat directory")
		case !dirStat.IsDir():
			return fmt.Errorf("the path %q already exists but is not a directory", dir)
		default:
			return checkAccess(dir, permBits)
		}
	}
}

func validateFile(file string, permBits uint32) error {
	switch {
	case !filepath.IsAbs(file):
		return fmt.Errorf("%q must be an absolute path", file)
	default:
		switch fileStat, err := os.Stat(file); {
		case os.IsNotExist(err):
			return errors.Wrapf(err, "file not found")
		case err != nil:
			return errors.Wrapf(err, "could not stat file")
		case fileStat.IsDir():
			return fmt.Errorf("the path %q already exists but is not a file", file)
		default:
			return checkAccess(file, permBits)
		}
	}
}

func (cfg *Config) ValidateVersionFile() error {
	if err := validateFile(cfg.VersionFile, unix.R_OK|unix.W_OK); err != nil {
		return errors.Wrapf(err, "error validating version-file")
	}
	if err := validateDir(path.Dir(cfg.VersionFile), unix.R_OK|unix.W_OK); err != nil {
		return errors.Wrapf(err, "error validating version-file")
	}
	return nil
}

func (cfg *Config) ValidateComposeFile() error {
	if err := validateFile(cfg.ComposeFile, unix.R_OK|unix.W_OK); err != nil {
		return errors.Wrapf(err, "error validating compose-file")
	}
	if err := validateDir(path.Dir(cfg.ComposeFile), unix.R_OK|unix.W_OK); err != nil {
		return errors.Wrapf(err, "error validating compose-file")
	}
	return nil
}

func (cfg *Config) ValidateChainHome() error {
	if err := validateDir(cfg.ChainHome, unix.R_OK|unix.W_OK); err != nil {
		return errors.Wrapf(err, "error validating chain-home")
	}
	// now check if the upgrades-info.json file is readable
	// if present, and if it is not present check if the data
	// dir is readable
	if err := validateFile(cfg.UpgradeInfoFilePath(), unix.R_OK); err != nil {
		if os.IsNotExist(errors.Unwrap(err)) {
			if err := validateDir(filepath.Join(cfg.ChainHome, "data"), unix.R_OK); err != nil {
				return errors.Wrapf(err, "error validating chain-home/data")
			}
		} else {
			return errors.Wrapf(err, "error validating chain-home/data/upgrade-info.json")
		}
	}
	return nil
}

func (cfg *Config) LoadWebhookURL() error {
	url := cfg.Slack.WebhookNotifier.WebhookURL
	if url[0] == '/' {
		// it must be a path
		contents, err := os.ReadFile(url)
		if err != nil {
			return errors.Wrapf(err, "failed reading %s file", url)
		}
		cfg.Slack.WebhookNotifier.WebhookURL = strings.TrimSpace(string(contents))
	}
	return nil
}

func (cfg *Config) LoadBotToken() error {
	token := cfg.Slack.BotNotifier.AuthToken
	if token[0] == '/' {
		// it must be a path
		contents, err := os.ReadFile(token)
		if err != nil {
			return errors.Wrapf(err, "failed reading %s file", token)
		}
		cfg.Slack.BotNotifier.AuthToken = strings.TrimSpace(string(contents))
	}
	return nil
}

func (cfg *Config) ValidateCredentialHelper() error {
	if err := validateFile(cfg.CredentialHelper.Command, unix.R_OK|unix.X_OK); err != nil {
		return errors.Wrapf(err, "error validating docker-credential-helper.command")
	}
	if cfg.CredentialHelper.Timeout <= 0 {
		return errors.New("docker-credential-helper.timeout cannot be less than or equal to 0")
	}
	return nil
}

func (cfg *Config) checkProvider(provider string) error {
	switch provider {
	case urproto.ProviderType_name[int32(urproto.ProviderType_CHAIN)]:
		if cfg.UpgradeRegistry.Provider.Chain == nil {
			return errors.New("upgrade-registry.provider.chain cannot be nil")
		}
	case urproto.ProviderType_name[int32(urproto.ProviderType_DATABASE)]:
		if cfg.UpgradeRegistry.Provider.Database == nil {
			return errors.New("upgrade-registry.provider.database cannot be nil")
		}
	case urproto.ProviderType_name[int32(urproto.ProviderType_LOCAL)]:
		if cfg.UpgradeRegistry.Provider.Local == nil {
			return errors.New("upgrade-registry.provider.local cannot be nil")
		}
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
	return nil
}

func (cfg *Config) ValidateBlazarHostGrpcPort() error {
	if cfg.Host == "" {
		return errors.New("host cannot be empty")
	}

	if cfg.GrpcPort == 0 {
		return errors.New("grpc-port cannot be 0")
	}
	return nil
}

func (cfg *Config) ValidateGrpcClient() error {
	if cfg.Clients.Host == "" {
		return errors.New("clients.host cannot be empty")
	}

	if cfg.Clients.GrpcPort == 0 {
		return errors.New("clients.grpc-port cannot be 0")
	}
	return nil
}

func (cfg *Config) ValidatePreUpgradeChecks() error {
	if cfg.Checks.PreUpgrade.Blocks <= 0 {
		return errors.New("checks.pre-upgrade.blocks cannot be less than 0")
	}
	for _, check := range cfg.Checks.PreUpgrade.Enabled {
		switch check {
		case checksproto.PreCheck_name[int32(checksproto.PreCheck_SET_HALT_HEIGHT)]:
			// there is no config so nothing to check
			if cfg.Checks.PreUpgrade.SetHaltHeight.DelayBlocks < 0 {
				return errors.New("checks.pre-upgrade.set-halt-height cannot be less than 0")
			}
		case checksproto.PreCheck_name[int32(checksproto.PreCheck_PULL_DOCKER_IMAGE)]:
			// there is no config so nothing to check
		default:
			return fmt.Errorf("unknown value in checks.pre-upgrade.enabled: %s", check)
		}
	}

	return nil
}

func (cfg *Config) ValidatePostUpgradeChecks() error {
	for _, check := range cfg.Checks.PostUpgrade.Enabled {
		switch check {
		case checksproto.PostCheck_name[int32(checksproto.PostCheck_GRPC_RESPONSIVE)]:
			if cfg.Checks.PostUpgrade.GrpcResponsive == nil {
				return errors.New("checks.post-upgrade.grpc-responsive cannot be nil")
			}
			if cfg.Checks.PostUpgrade.GrpcResponsive.PollInterval <= 0 {
				return errors.New("checks.post-upgrade.grpc-responsive.poll-interval cannot be less than or equal to 0")
			}
			if cfg.Checks.PostUpgrade.GrpcResponsive.Timeout <= 0 {
				return errors.New("checks.post-upgrade.grpc-responsive.timeout cannot be less than or equal to 0")
			}
		case checksproto.PostCheck_name[int32(checksproto.PostCheck_CHAIN_HEIGHT_INCREASED)]:
			if cfg.Checks.PostUpgrade.ChainHeightIncreased == nil {
				return errors.New("checks.post-upgrade.chain-height-increased cannot be nil")
			}
			if cfg.Checks.PostUpgrade.ChainHeightIncreased.PollInterval <= 0 {
				return errors.New("checks.post-upgrade.chain-height-increased.poll-interval cannot be less than or equal to 0")
			}
			if cfg.Checks.PostUpgrade.ChainHeightIncreased.NotifInterval <= 0 {
				return errors.New("checks.post-upgrade.chain-height-increased.notif-interval cannot be less than or equal to 0")
			}
			if cfg.Checks.PostUpgrade.ChainHeightIncreased.Timeout <= 0 {
				return errors.New("checks.post-upgrade.chain-height-increased.timeout cannot be less than or equal to 0")
			}
		case checksproto.PostCheck_name[int32(checksproto.PostCheck_FIRST_BLOCK_VOTED)]:
			if cfg.Checks.PostUpgrade.FirstBlockVoted == nil {
				return errors.New("checks.post-upgrade.first-block-voted cannot be nil")
			}
			if cfg.Checks.PostUpgrade.FirstBlockVoted.PollInterval <= 0 {
				return errors.New("checks.post-upgrade.first-block-voted cannot be less than or equal to 0")
			}
			if cfg.Checks.PostUpgrade.FirstBlockVoted.NotifInterval <= 0 {
				return errors.New("checks.post-upgrade.first-block-voted cannot be less than or equal to 0")
			}
			if cfg.Checks.PostUpgrade.FirstBlockVoted.Timeout <= 0 {
				return errors.New("checks.post-upgrade.first-block-voted cannot be less than or equal to 0")
			}
		default:
			return fmt.Errorf("unknown value in checks.post-upgrade.enabled: %s", check)
		}
	}

	return nil
}

func (cfg *Config) ValidateAll() error {
	if err := cfg.ValidateComposeFile(); err != nil {
		return err
	}

	if cfg.ComposeService == "" {
		return errors.New("compose-service cannot be empty")
	}

	if !slices.Contains(ValidUpgradeModes, cfg.UpgradeMode) {
		return fmt.Errorf("invalid upgradeMode '%s', pick one of %+v", cfg.UpgradeMode, ValidUpgradeModes)
	}
	if cfg.UpgradeMode == "env-file" {
		if err := cfg.ValidateVersionFile(); err != nil {
			return err
		}
	}

	if err := cfg.ValidateChainHome(); err != nil {
		return err
	}

	if cfg.ChainID == "" {
		return errors.New("chain-id cannot be empty")
	}

	if cfg.LogLevel < -1 || cfg.LogLevel > 7 {
		return errors.New("log-level must be between -1 and 7, refer https://pkg.go.dev/github.com/rs/zerolog#readme-leveled-logging for more info")
	}

	if err := cfg.ValidateBlazarHostGrpcPort(); err != nil {
		return err
	}

	if cfg.HTTPPort == 0 {
		return errors.New("http-port cannot be 0")
	}

	if cfg.Watchers.UIInterval < 0 {
		return errors.New("watchers.upgrade-info-interval cannot be less than 0")
	}

	if cfg.Watchers.HInterval < 0 {
		return errors.New("watchers.height-interval cannot be less than 0")
	}

	if cfg.Watchers.HInterval == 0 && cfg.Watchers.HTimeout <= 0 {
		return errors.New("watchers.height-timeout cannot be less than or equal to 0 when using ws subscriptions")
	}

	if cfg.Watchers.UPInterval <= 0 {
		return errors.New("watchers.upgrade-proposals-interval cannot be less than or equal to 0")
	}

	if err := cfg.ValidateGrpcClient(); err != nil {
		return nil
	}

	if cfg.Clients.CometbftPort == 0 {
		return errors.New("clients.cometbft-port cannot be 0")
	}

	if cfg.Clients.Timeout <= 0 {
		return errors.New("clients.timeout cannot be less than or equal to 0")
	}

	if cfg.Compose.DownTimeout < 10*time.Second {
		return errors.New("compose-cli.down-timeout cannot be less than 10s")
	}

	if cfg.Compose.UpDeadline < 10*time.Second {
		return errors.New("compose-cli.up-deadline cannot be less than 10s")
	}

	if err := cfg.ValidatePreUpgradeChecks(); err != nil {
		return err
	}

	if err := cfg.ValidatePostUpgradeChecks(); err != nil {
		return err
	}

	// slack notifications are not mandatory
	if cfg.Slack != nil {
		if cfg.Slack.WebhookNotifier != nil && cfg.Slack.BotNotifier != nil {
			return errors.New("there can only be one slack notifier, please choose one webhook or bot notifier")
		}

		if cfg.Slack.WebhookNotifier != nil {
			if cfg.Slack.WebhookNotifier.WebhookURL == "" {
				return errors.New("slack.webhook-notifier.webhook-url cannot be empty")
			}
			if err := cfg.LoadWebhookURL(); err != nil {
				return err
			}
		}

		if cfg.Slack.BotNotifier != nil {
			if cfg.Slack.BotNotifier.AuthToken == "" {
				return errors.New("slack.bot-notifier.auth-token cannot be empty")
			}
			if cfg.Slack.BotNotifier.Channel == "" {
				return errors.New("slack.bot-notifier.channel cannot be empty")
			}
			if err := cfg.LoadBotToken(); err != nil {
				return err
			}
		}
	}

	// docker credential helper is not mandatory
	if cfg.CredentialHelper != nil {
		if err := cfg.ValidateCredentialHelper(); err != nil {
			return err
		}
	}

	if len(cfg.UpgradeRegistry.SelectedProviders) == 0 {
		return errors.New("upgrade-registry.providers cannot be empty")
	}
	for i, provider := range cfg.UpgradeRegistry.SelectedProviders {
		provider = strings.ToUpper(provider)
		cfg.UpgradeRegistry.SelectedProviders[i] = provider

		if err := cfg.checkProvider(provider); err != nil {
			return errors.Wrapf(err, "error validating upgrade-registry.providers")
		}
	}

	if cfg.UpgradeRegistry.Network == "" {
		return errors.New("upgrade-registry.network cannot be empty")
	}

	if cfg.UpgradeRegistry.Provider.Chain != nil {
		if cfg.UpgradeRegistry.Provider.Chain.DefaultPriority < 1 || cfg.UpgradeRegistry.Provider.Chain.DefaultPriority > 99 {
			return errors.New("upgrade-registry.provider.chain.default-priority must be between 1 and 99")
		}
	}

	if cfg.UpgradeRegistry.Provider.Database != nil {
		if cfg.UpgradeRegistry.Provider.Database.DefaultPriority < 1 || cfg.UpgradeRegistry.Provider.Database.DefaultPriority > 99 {
			return errors.New("upgrade-registry.provider.database.priority must be between 1 and 99")
		}
		if cfg.UpgradeRegistry.Provider.Database.Host == "" {
			return errors.New("upgrade-registry.provider.database.host cannot be empty")
		}
		if cfg.UpgradeRegistry.Provider.Database.Port == 0 {
			return errors.New("upgrade-registry.provider.database.port cannot be empty")
		}
		if cfg.UpgradeRegistry.Provider.Database.DB == "" {
			return errors.New("upgrade-registry.provider.database.db cannot be empty")
		}
		if cfg.UpgradeRegistry.Provider.Database.User == "" {
			return errors.New("upgrade-registry.provider.database.user cannot be empty")
		}
		if cfg.UpgradeRegistry.Provider.Database.PasswordFile != "" {
			if err := validateFile(cfg.UpgradeRegistry.Provider.Database.PasswordFile, unix.R_OK); err != nil {
				return errors.Wrapf(err, "error validating upgrade-registry.provider.database.password-file")
			}
			contents, err := os.ReadFile(cfg.UpgradeRegistry.Provider.Database.PasswordFile)
			if err != nil {
				return errors.Wrapf(err, "failed to open file: %s", cfg.UpgradeRegistry.Provider.Database.PasswordFile)
			}
			cfg.UpgradeRegistry.Provider.Database.Password = strings.TrimSpace(string(contents))
		} else if cfg.UpgradeRegistry.Provider.Database.Password == "" {
			return errors.New("upgrade-registry.provider.database.password cannot be empty")
		}

		switch cfg.UpgradeRegistry.Provider.Database.SslMode {
		case Disable, Allow, Prefer, Require, VerifyCa, VerifyFull:
		default:
			return errors.New("upgrade-registry.provider.database.ssl-mode must be one of disable, allow, prefer, require, verify-ca, verify-full")
		}
	}

	if cfg.UpgradeRegistry.Provider.Local != nil {
		if cfg.UpgradeRegistry.Provider.Local.DefaultPriority < 1 || cfg.UpgradeRegistry.Provider.Local.DefaultPriority > 99 {
			return errors.New("upgrade-registry.provider.local.priority must be between 1 and 99")
		}
		if cfg.UpgradeRegistry.Provider.Local.ConfigPath == "" {
			return errors.New("upgrade-registry.provider.local.config-path cannot be empty")
		}
	}

	// version resolver is optional
	if cfg.UpgradeRegistry.VersionResolvers != nil {
		if len(cfg.UpgradeRegistry.VersionResolvers.Providers) == 0 {
			return errors.New("upgrade-registry.version-resolvers.providers cannot be empty")
		}
		for i, provider := range cfg.UpgradeRegistry.VersionResolvers.Providers {
			provider = strings.ToUpper(provider)
			cfg.UpgradeRegistry.VersionResolvers.Providers[i] = provider

			if err := cfg.checkProvider(provider); err != nil {
				return errors.Wrapf(err, "error validating upgrade-registry.version-resolvers.providers")
			}
		}
	}

	cfg.UpgradeRegistry.StateMachine.Provider = strings.ToUpper(cfg.UpgradeRegistry.StateMachine.Provider)
	if err := cfg.checkProvider(cfg.UpgradeRegistry.StateMachine.Provider); err != nil {
		return errors.Wrapf(err, "error validating upgrade-registry.state-machine.provider")
	}

	return nil
}
