package versions

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"blazar/cmd/util"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/log/logger"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	proto "blazar/internal/pkg/proto/version_resolver"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	timeout time.Duration
	noCache bool

	filterHeight       int64
	filterProviderType string
)

func GetVersionsListCmd() *cobra.Command {
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

			c := proto.NewVersionResolverClient(conn)

			filterProviderType, filterHeight, err := filterFlags()
			if err != nil {
				return err
			}

			listUpgradesResponse, err := c.ListVersions(ctx, &proto.ListVersionsRequest{
				DisableCache: noCache,
				Height:       filterHeight,
				Source:       filterProviderType,
			})
			if err != nil {
				return err
			}

			tw := table.NewWriter()
			tw.AppendHeader(table.Row{
				"Height",
				"Tag",
				"Network",
				"Priority",
				"Source",
			})

			for _, version := range listUpgradesResponse.Versions {
				tw.AppendRow(table.Row{
					version.Height,
					version.Tag,
					version.Network,
					version.GetPriority(),
					version.Source,
				})
			}

			fmt.Println(tw.Render())

			return nil
		},
	}

	listCmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Grpc request timeout")
	listCmd.Flags().BoolVar(&noCache, "nocache", false, "Skip upgrade registry cache (slower but more accurate)")
	listCmd.Flags().Int64Var(&filterHeight, "height", 0, "Filter by height")
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

func filterFlags() (*urproto.ProviderType, *int64, error) {
	var requestedProviderType *urproto.ProviderType
	if filterProviderType != "" {
		if _, ok := urproto.ProviderType_value[filterProviderType]; !ok {
			return nil, nil, fmt.Errorf("invalid provider type type: %s", filterProviderType)
		}
		value := urproto.ProviderType(urproto.ProviderType_value[filterProviderType])
		requestedProviderType = &value
	}

	var requestedHeight *int64
	if filterHeight != 0 {
		requestedHeight = &filterHeight
	}

	return requestedProviderType, requestedHeight, nil
}
