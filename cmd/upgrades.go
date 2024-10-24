package cmd

import (
	"blazar/cmd/upgrades"

	"github.com/spf13/cobra"
)

var upgradesCmd = &cobra.Command{
	Use:   "upgrades",
	Short: "Upgrades related commands",
}

func init() {
	upgradesCmd.AddCommand(upgrades.GetUpgradeListCmd())
	upgradesCmd.AddCommand(upgrades.GetUpgradeRegisterCmd())
	upgradesCmd.AddCommand(upgrades.GetForceSyncCmd())

	upgradesCmd.PersistentFlags().String("host", "", "Blazar host to talk to, will override config values if config is specified")
	upgradesCmd.PersistentFlags().Uint16("port", 0, "Blazar grpc port to talk to, will override config values if config is specified")

	rootCmd.AddCommand(upgradesCmd)
}
