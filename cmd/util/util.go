package util

import (
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/errors"

	"github.com/spf13/cobra"
)

func GetBlazarHostPort(cmd *cobra.Command, cfg *config.Config) (string, uint16, error) {
	var host string
	var port uint16

	if cfg != nil {
		if err := cfg.ValidateBlazarHostGrpcPort(); err != nil {
			return "", 0, err
		}
		host, port = cfg.Host, cfg.GrpcPort
	}

	blazarGrpcPort, err := cmd.Flags().GetUint16("port")
	if err != nil {
		// this should never be hit
		panic(err)
	}
	if blazarGrpcPort != 0 {
		port = blazarGrpcPort
	}

	blazarHost, err := cmd.Flags().GetString("host")
	if err != nil {
		// this should never be hit
		panic(err)
	}
	if blazarHost != "" {
		host = blazarHost
	}

	if port == 0 {
		return "", 0, errors.New("blazar grpc port not specified")
	}

	if host == "" {
		return "", 0, errors.New("blazar host not specified")
	}
	return host, port, nil
}
