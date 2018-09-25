package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

type VersionInfo struct {
	Version, Commit, Date string
}

var versionInfo VersionInfo

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%v, commit %v, built at %v\n", versionInfo.Version, versionInfo.Commit, versionInfo.Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
