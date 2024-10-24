package cmd

import (
	"blazar/cmd/provider"

	"github.com/spf13/cobra"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Provider related commands",
}

func init() {
	providerCmd.AddCommand(provider.GetProviderDatabaseCmd())
	rootCmd.AddCommand(providerCmd)
}
