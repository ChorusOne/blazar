package database

import (
	"fmt"

	"blazar/internal/pkg/config"
	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/provider/database"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func GetDatabaseMigrationsCmd() *cobra.Command {
	registerMigrationCmd := &cobra.Command{
		Use:   "migration",
		Short: "Database migrations related commands",
	}

	registerMigrationCmd.AddCommand(GetMigrationDumpCmd())
	registerMigrationCmd.AddCommand(GetMigrationApplyCmd())

	return registerMigrationCmd
}

func GetMigrationDumpCmd() *cobra.Command {
	registerDumpCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump SQL changes to the screen",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := readConfig(cmd)
			if err != nil {
				return err
			}

			dbCfg := cfg.UpgradeRegistry.Provider.Database

			db, err := database.InitDB(dbCfg, &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			if err != nil {
				return err
			}

			tx := db.Begin()
			var statements []string
			if err := tx.Callback().Raw().Register("record_blazar_migration", func(tx *gorm.DB) {
				statements = append(statements, tx.Statement.SQL.String())
			}); err != nil {
				return err
			}
			if err := database.AutoMigrate(tx); err != nil {
				return err
			}
			tx.Rollback()

			if err := tx.Callback().Raw().Remove("record_blazar_migration"); err != nil {
				return err
			}

			for _, s := range statements {
				fmt.Println(s)
			}

			return nil
		},
	}

	return registerDumpCmd
}

func GetMigrationApplyCmd() *cobra.Command {
	registerUCmd := &cobra.Command{
		Use:   "apply",
		Short: "Perform database auto-migration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := readConfig(cmd)
			if err != nil {
				return err
			}

			dbCfg := cfg.UpgradeRegistry.Provider.Database
			db, err := database.InitDB(dbCfg, &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			if err != nil {
				return err
			}

			if err := database.AutoMigrate(db); err != nil {
				return errors.Wrapf(err, "failed to auto-migrate database")
			}

			fmt.Println("Database auto-migration successful")
			return nil
		},
	}

	return registerUCmd
}

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
