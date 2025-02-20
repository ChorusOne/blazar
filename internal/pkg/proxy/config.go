package proxy

import (
	"blazar/internal/pkg/errors"

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
	Host      string     `toml:"host"`
	HTTPPort  uint16     `toml:"http-port"`
	Instances []Instance `toml:"instance"`
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
