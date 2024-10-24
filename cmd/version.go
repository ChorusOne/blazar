package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"blazar/internal/pkg/log/logger"

	"github.com/spf13/cobra"
)

var (
	BinVersion     = "unknown"
	GitStatus      = "unknown"
	GitCommit      = "unknown"
	BuildTime      = "unknown"
	BuildGoVersion = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Version of the blazar binary",
	Run: func(_ *cobra.Command, _ []string) {
		logger.NewLogger().Info().Msgf("Blazar version:\n%s", getVersion())
	},
}

func getVersion() string {
	return fmt.Sprintf("Version=%s\nGitStatus=%s\nGitCommit=%s\nBuildTime=%s\nBuildWith=%s\nRunOn=%s/%s\n",
		BinVersion, GitStatus, GitCommit, BuildTime, BuildGoVersion, runtime.GOOS, runtime.GOARCH)
}

func init() {
	rootCmd.AddCommand(versionCmd)
	if GitStatus == "" {
		GitStatus = "up to date"
	} else {
		GitStatus = strings.ReplaceAll(strings.ReplaceAll(GitStatus, "\r\n", " | "), "\n", " | ")
	}
}
