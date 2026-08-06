package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information set at build time.
var (
	Version   = "0.26.08"
	BuildNum  = "0002"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// VersionString returns the full version string.
func VersionString() string {
	return Version + " build" + BuildNum
}

var VersionShort = "0.26.08"

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("msync version %s build%s\n", Version, BuildNum)
			fmt.Printf("  commit: %s\n", GitCommit)
			fmt.Printf("  built:  %s\n", BuildDate)
		},
	}
}
