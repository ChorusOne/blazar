package proxy

import (
	"blazar/internal/pkg/errors"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

type Instance struct {
	Name     string `toml:"name"`
	Host     string `toml:"host"`
	HTTPPort int    `toml:"http-port"`
	GRPCPort int    `toml:"grpc-port"`
	Network  string `toml:"network"`
}

type Config struct {
	Host         string        `toml:"host"`
	HTTPPort     uint16        `toml:"http-port"`
	PollInterval time.Duration `toml:"poll-interval"`
	Instances    []Instance    `toml:"instance"`
}

func ReadConfig(cfgFile string) (*Config, error) {
	var config Config
	_, err := toml.DecodeFile(cfgFile, &config)
	if err != nil {
		return nil, errors.Wrapf(err, "could not decode config file")
	}
	return &config, nil
}

func (cfg *Config) ValidateAll() error {
	if len(cfg.Instances) == 0 {
		return errors.New("no instances specified")
	}
	if cfg.PollInterval <= time.Duration(0) {
		return errors.New(fmt.Sprintf("poll interval not specified or invalid value, got value: %s", cfg.PollInterval))
	}
	if cfg.Host == "" {
		return errors.New("listen host not specified")
	}
	if cfg.HTTPPort == 0 {
		return errors.New("listen port not specified")
	}

	for _, instance := range cfg.Instances {
		if instance.Name == "" {
			return errors.New("instance name not specified")
		}
		if instance.Host == "" {
			return errors.New("instance host not specified")
		}
		if instance.HTTPPort == 0 {
			return errors.New("instance http port not specified")
		}
		if instance.GRPCPort == 0 {
			return errors.New("instance grpc port not specified")
		}
	}

	return nil
}
