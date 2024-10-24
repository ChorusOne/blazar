package versions

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"blazar/cmd/util"
	"blazar/internal/pkg/config"
	"blazar/internal/pkg/log/logger"
	urproto "blazar/internal/pkg/proto/upgrades_registry"
	vrproto "blazar/internal/pkg/proto/version_resolver"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Version fields
	height   int64
	tag      string
	priority int32
	source   string

	// Version request fields
	overwrite bool

	// Other
	allUpgradeSources []string
)

func init() {
	for _, source := range urproto.ProviderType_name {
		allUpgradeSources = append(allUpgradeSources, source)
	}
}

func GetVersionRegisterCmd() *cobra.Command {
	registerUpgradeCmd := &cobra.Command{
		Use:   "register",
		Short: "Tell blazar what image tag to upgrade to when it detects an upgrade at the specified chain height",
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

			c := vrproto.NewVersionResolverClient(conn)

			// handle upgrade fields
			if _, ok := urproto.ProviderType_value[source]; !ok {
				return fmt.Errorf("invalid source: %s", source)
			}

			upgrade := &vrproto.Version{
				Height:   height,
				Tag:      tag,
				Priority: priority,
				Source:   urproto.ProviderType(urproto.ProviderType_value[source]),
			}

			if _, err = c.AddVersion(ctx, &vrproto.RegisterVersionRequest{
				Version:   upgrade,
				Overwrite: overwrite,
			}); err != nil {
				return err
			}
			lg.Info().Msgf("Successfully registered version for height=%d tag=%s", height, tag)
			return nil
		},
	}

	registerUpgradeCmd.Flags().Int64Var(&height, "height", 0, "Height to register upgrade for")
	registerUpgradeCmd.Flags().StringVar(&tag, "tag", "", "Tag to upgrade to")
	registerUpgradeCmd.Flags().Int32Var(&priority, "priority", 0, "Upgrade priority")
	registerUpgradeCmd.Flags().StringVar(
		&source, "source", "",
		fmt.Sprintf("Upgrade source; valid values: %s", strings.Join(allUpgradeSources, ", ")),
	)
	registerUpgradeCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing upgrade")

	for _, flagName := range []string{"height", "tag", "source"} {
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
