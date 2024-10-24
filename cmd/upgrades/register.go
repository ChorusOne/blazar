package upgrades

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"blazar/cmd/util"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/log/logger"
	blazarproto "blazar/internal/pkg/proto/blazar"
	urproto "blazar/internal/pkg/proto/upgrades_registry"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Upgrade fields
	height      string
	tag         string
	name        string
	upgradeType string
	priority    int32
	source      string
	proposalID  int64

	// Upgrade request fields
	overwrite bool

	// Other
	allUpgradeTypes   []string
	allUpgradeSources []string
)

func init() {
	for _, upgradeType := range urproto.UpgradeType_name {
		allUpgradeTypes = append(allUpgradeTypes, upgradeType)
	}
	for _, source := range urproto.ProviderType_name {
		allUpgradeSources = append(allUpgradeSources, source)
	}
}

func GetUpgradeRegisterCmd() *cobra.Command {
	registerUpgradeCmd := &cobra.Command{
		Use:   "register",
		Short: "Associate image tag with an upgrade at a specific height",
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
			if _, ok := urproto.UpgradeType_value[upgradeType]; !ok {
				return fmt.Errorf("invalid upgrade type: %s", upgradeType)
			}

			if _, ok := urproto.ProviderType_value[source]; !ok {
				return fmt.Errorf("invalid source: %s", source)
			}

			if height == "" {
				return fmt.Errorf("height is required")
			}

			// handle human friendly syntax for height
			// +100 means 100 blocks from now
			// 100 means block 100
			upgradeHeight := int64(0)
			if height[0] == '+' {
				b := blazarproto.NewBlazarClient(conn)
				heightResponse, err := b.GetLastestHeight(ctx, &blazarproto.GetLatestHeightRequest{})
				if err != nil {
					return err
				}
				latestHeight := heightResponse.GetHeight()

				heightOffset, err := strconv.ParseInt(height[1:], 10, 64)
				if err != nil {
					return err
				}
				upgradeHeight = latestHeight + heightOffset
			} else {
				upgradeHeight, err = strconv.ParseInt(height, 10, 64)
				if err != nil {
					return err
				}
			}

			upgrade := &urproto.Upgrade{
				Height: upgradeHeight,
				Tag:    tag,
				Name:   name,
				Type:   urproto.UpgradeType(urproto.UpgradeType_value[upgradeType]),
				// Status is managed by the blazar registry
				Status:     urproto.UpgradeStatus_UNKNOWN,
				Priority:   priority,
				Source:     urproto.ProviderType(urproto.ProviderType_value[source]),
				ProposalId: nil,
			}

			if proposalID != -1 {
				upgrade.ProposalId = &proposalID
			}

			serialized, err := json.MarshalIndent(&upgrade, "", "  ")
			if err != nil {
				return err
			}

			lg.Info().Msgf("Registering upgrade: %s", string(serialized))

			if _, err = c.AddUpgrade(ctx, &urproto.AddUpgradeRequest{
				Upgrade:   upgrade,
				Overwrite: overwrite,
			}); err != nil {
				return err
			}
			lg.Info().Msgf("Successfully registered upgrade for height=%s tag=%s", height, tag)
			return nil
		},
	}

	registerUpgradeCmd.Flags().StringVar(&height, "height", "", "Height to register upgrade for (1234 or +100 for 100 blocks from now)")
	registerUpgradeCmd.Flags().StringVar(&tag, "tag", "", "Tag to upgrade to")
	registerUpgradeCmd.Flags().StringVar(&name, "name", "", "A short text describing the upgrade")
	registerUpgradeCmd.Flags().StringVar(
		&upgradeType, "type", "",
		fmt.Sprintf("Upgrade type; valid values: %s", strings.Join(allUpgradeTypes, ", ")),
	)
	registerUpgradeCmd.Flags().Int32Var(&priority, "priority", 0, "Upgrade priority")
	registerUpgradeCmd.Flags().StringVar(
		&source, "source", "",
		fmt.Sprintf("Upgrade source; valid values: %s", strings.Join(allUpgradeSources, ", ")),
	)
	registerUpgradeCmd.Flags().Int64Var(&proposalID, "proposal-id", -1, "Proposal ID")
	registerUpgradeCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing upgrade")

	for _, flagName := range []string{"height", "tag", "type", "source"} {
		err := registerUpgradeCmd.MarkFlagRequired(flagName)
		cobra.CheckErr(err)
	}

	return registerUpgradeCmd
}

// Read the cfg if it is specified in flags
func readConfig(cmd *cobra.Command) (*config.Config, error) {
	cfgFile := cmd.Flag("config").Value.String()
	if cfgFile != "" {
		cfg, err := config.ReadConfig(cfgFile)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return nil, nil
}
