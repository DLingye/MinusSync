package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information set at build time.
var (
	Version   = "1.0.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("msync version %s\n", Version)
			fmt.Printf("  commit: %s\n", GitCommit)
			fmt.Printf("  built:  %s\n", BuildDate)
		},
	}
}
