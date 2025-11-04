package upgrades

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"blazar/cmd/util"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/log/logger"
	blazarproto "blazar/internal/pkg/proto/blazar"
	proto "blazar/internal/pkg/proto/upgrades_registry"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	timeout time.Duration
	noCache bool

	filterHeight       int64
	filterUpgradeType  string
	filterProviderType string
)

func GetUpgradeListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all upgrades",
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

			if err := parseConfig(cfg); err != nil {
				return err
			}

			addr := net.JoinHostPort(host, strconv.Itoa(int(port)))
			conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}

			b := blazarproto.NewBlazarClient(conn)
			heightResponse, err := b.GetLastestHeight(ctx, &blazarproto.GetLatestHeightRequest{})
			latestHeight := int64(0)
			if err != nil {
				// We don't return error because if the upgrade is ongoing, the node is expected to be down and return error.
				// In such case we just show UNKNOWN
				lg.Warn().Err(err).Msg("Failed to get latest height")
			} else {
				latestHeight = heightResponse.GetHeight()
			}

			requestedUpgradeType, requestedProviderType, requestedHeight, err := filterFlags()
			if err != nil {
				return err
			}

			c := proto.NewUpgradeRegistryClient(conn)
			listUpgradesResponse, err := c.ListUpgrades(ctx, &proto.ListUpgradesRequest{
				DisableCache: noCache,
				Height:       requestedHeight,
				Type:         requestedUpgradeType,
				Source:       requestedProviderType,
			})
			if err != nil {
				return err
			}

			tw := table.NewWriter()
			tw.AppendHeader(table.Row{
				"Height",
				"Tag",
				"Network",
				"Name",
				"Type",
				"Status",
				"Step",
				"Priority",
				"Source",
				"ProposalID",
				"Blocks_to_upgrade",
				"Created_at",
			})

			for _, upgrade := range listUpgradesResponse.Upgrades {
				blocksToUpgrade := ""
				if latestHeight != 0 {
					blocksToUpgrade = strconv.FormatInt(upgrade.GetHeight()-latestHeight, 10)
				}

				tw.AppendRow(table.Row{
					upgrade.Height,
					upgrade.Tag,
					upgrade.Network,
					upgrade.Name,
					upgrade.Type,
					upgrade.Status,
					upgrade.Step,
					upgrade.GetPriority(),
					upgrade.Source,
					upgrade.GetProposalId(),
					blocksToUpgrade,
					upgrade.CreatedAt,
				})
			}

			fmt.Println(tw.Render())

			return nil
		},
	}

	listCmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Grpc request timeout")
	listCmd.Flags().BoolVar(&noCache, "nocache", false, "Skip upgrade registry cache (slower but more accurate)")
	listCmd.Flags().Int64Var(&filterHeight, "height", 0, "Filter by height")
	listCmd.Flags().StringVar(&filterUpgradeType, "type", "", "Filter by upgrade type")
	listCmd.Flags().StringVar(&filterProviderType, "provider", "", "Filter by provider type")

	return listCmd
}

func parseConfig(cfg *config.Config) error {
	if cfg != nil {
		if err := cfg.ValidateBlazarHostGrpcPort(); err != nil {
			return err
		}
	}
	return nil
}

func filterFlags() (*proto.UpgradeType, *proto.ProviderType, *int64, error) {
	var requestedUpgradeType *proto.UpgradeType
	if filterUpgradeType != "" {
		if _, ok := proto.UpgradeType_value[filterUpgradeType]; !ok {
			return nil, nil, nil, fmt.Errorf("invalid upgrade type: %s", filterUpgradeType)
		}
		value := proto.UpgradeType(proto.UpgradeType_value[filterUpgradeType])
		requestedUpgradeType = &value
	}

	var requestedProviderType *proto.ProviderType
	if filterProviderType != "" {
		if _, ok := proto.ProviderType_value[filterProviderType]; !ok {
			return nil, nil, nil, fmt.Errorf("invalid provider type type: %s", filterProviderType)
		}
		value := proto.ProviderType(proto.ProviderType_value[filterProviderType])
		requestedProviderType = &value
	}

	var requestedHeight *int64
	if filterHeight != 0 {
		requestedHeight = &filterHeight
	}

	return requestedUpgradeType, requestedProviderType, requestedHeight, nil
}
