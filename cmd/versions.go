package cmd

import (
	"blazar/cmd/versions"

	"github.com/spf13/cobra"
)

var versionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "Versions related commands",
}

func init() {
	versionsCmd.AddCommand(versions.GetVersionsListCmd())
	versionsCmd.AddCommand(versions.GetVersionRegisterCmd())

	versionsCmd.PersistentFlags().String("host", "", "Blazar host to talk to, will override config values if config is specified")
	versionsCmd.PersistentFlags().Uint16("port", 0, "Blazar grpc port to talk to, will override config values if config is specified")

	rootCmd.AddCommand(versionsCmd)
}
