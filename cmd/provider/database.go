package provider

import (
	"blazar/cmd/provider/database"

	"github.com/spf13/cobra"
)

func GetProviderDatabaseCmd() *cobra.Command {
	registerDatabaseCmd := &cobra.Command{
		Use:   "database",
		Short: "Database provider related commands",
	}

	registerDatabaseCmd.AddCommand(database.GetDatabaseMigrationsCmd())

	return registerDatabaseCmd
}
