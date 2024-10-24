package upgrades

import (
	"fmt"
	"net"
	"strconv"

	"blazar/cmd/util"
	"blazar/internal/pkg/log/logger"
	proto "blazar/internal/pkg/proto/upgrades_registry"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func GetForceSyncCmd() *cobra.Command {
	forceSyncCmd := &cobra.Command{
		Use:   "force-sync",
		Short: "Send a force sync request to synchronise the registry with latest data",
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

			c := proto.NewUpgradeRegistryClient(conn)
			response, err := c.ForceSync(ctx, &proto.ForceSyncRequest{})
			if err != nil {
				return err
			}

			fmt.Printf("Update registry synchronised successfully at height: %d\n", response.Height)

			return nil
		},
	}

	return forceSyncCmd
}
