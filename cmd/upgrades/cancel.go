package upgrades

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"blazar/cmd/util"
	"blazar/internal/pkg/log/logger"
	urproto "blazar/internal/pkg/proto/upgrades_registry"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Update state machine directly
	force bool
)

func GetUpgradeCancelCmd() *cobra.Command {
	cancelUpgradeCmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an upgrade",
		RunE: func(cmd *cobra.Command, _ []string) error {
			lg := logger.NewLogger()
			ctx := logger.WithContext(cmd.Context(), lg)

			cfg, err := readConfig(cmd)
			if err != nil {
				return err
			}

			host, port, err := util.GetBlazarHostPort(cmd, cfg)
			if err != nil {
				return err
			}

			addr := net.JoinHostPort(host, strconv.Itoa(int(port)))
			conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}

			c := urproto.NewUpgradeRegistryClient(conn)

			// handle upgrade fields
			if _, ok := urproto.ProviderType_value[source]; !ok {
				return fmt.Errorf("invalid source: %s", source)
			}

			if height == "" {
				return fmt.Errorf("height is required")
			}

			upgradeHeight, err := strconv.ParseInt(height, 10, 64)
			if err != nil {
				return err
			}

			cancelRequest := &urproto.CancelUpgradeRequest{
				Height: upgradeHeight,
				Source: urproto.ProviderType(urproto.ProviderType_value[source]),
			}

			serialized, err := json.MarshalIndent(&cancelRequest, "", "  ")
			if err != nil {
				return err
			}

			lg.Info().Msgf("Cancelling upgrade: %s", string(serialized))

			if _, err = c.CancelUpgrade(ctx, cancelRequest); err != nil {
				return err
			}
			lg.Info().Msgf("Successfully cancelled upgrade for height=%s", height)
			return nil
		},
	}

	cancelUpgradeCmd.Flags().StringVar(&height, "height", "", "Height to register upgrade for (1234 or +100 for 100 blocks from now)")
	cancelUpgradeCmd.Flags().StringVar(
		&source, "source", "",
		fmt.Sprintf("Upgrade source; valid values: %s", strings.Join(allUpgradeSources, ", ")),
	)
	cancelUpgradeCmd.Flags().BoolVar(&force, "force", false, "Forcefully set the state machine status to CANCELLED")

	for _, flagName := range []string{"height", "network", "source"} {
		err := cancelUpgradeCmd.MarkFlagRequired(flagName)
		cobra.CheckErr(err)
	}

	return cancelUpgradeCmd
}
