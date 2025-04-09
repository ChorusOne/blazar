package cmd

import (
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/daemon"
	"blazar/internal/pkg/daemon/util"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log/logger"
	"blazar/internal/pkg/log/notification"
	"blazar/internal/pkg/metrics"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the blazar daemon",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfgFile := cmd.Flag("config").Value.String()
		cfg, err := config.ReadConfig(cfgFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read the toml config")
		}

		if err := cfg.ValidateAll(); err != nil {
			return errors.Wrapf(err, "failed to validate config")
		}

		// setup logging level
		logger.SetGlobalLogLevel(cfg.LogLevel)

		// setup initial logger
		lg := logger.FromContext(cmd.Context())

		// setup metrics
		hostname := util.GetHostname()
		metrics := metrics.NewMetrics(cfg.ComposeFile, hostname, BinVersion, cfg.ChainID)

		// setup notifier
		notifier := notification.NewFallbackNotifier(cfg, metrics, lg, hostname)

		// setup daemon
		d, err := daemon.NewDaemon(cmd.Context(), cfg, metrics)
		if err != nil {
			return errors.Wrapf(err, "failed to setup new daemon")
		}

		if err := d.ListenAndServe(cmd.Context(), cfg); err != nil {
			return errors.Wrapf(err, "failed to start grpc/http server")
		}

		// setup notifier in the context
		ctx := notification.WithContextFallback(cmd.Context(), notifier)

		// initialize daemon (fetch initial state and run basic sanity checks)
		if err := d.Init(ctx, cfg); err != nil {
			return errors.Wrapf(err, "failed to initialize daemon")
		}

		// start the daemon (monitor and run any upcoming upgrade)
		if err := d.Run(ctx, cfg); err != nil {
			return errors.Wrapf(err, "daemon run failed")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
