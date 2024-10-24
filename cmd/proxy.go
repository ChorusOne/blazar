package cmd

import (
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/proxy"

	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the Blazar proxy daemon",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfgFile := cmd.Flag("config").Value.String()
		cfg, err := proxy.ReadConfig(cfgFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read the toml config")
		}

		if err := cfg.ValidateAll(); err != nil {
			return errors.Wrapf(err, "failed to validate config")
		}

		// setup daemon
		d := proxy.NewProxy()
		if err := d.ListenAndServe(cmd.Context(), cfg); err != nil {
			return errors.Wrapf(err, "failed to start grpc/http server")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(proxyCmd)
}
