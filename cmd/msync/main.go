// MinusSync (msync) — A version control and file synchronization system.
package main

import (
	"fmt"
	"os"

	"github.com/MinusSync/internal/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "msync",
	Short: "MinusSync v" + cmd.Version + " Build" + cmd.BuildNum + " — version control and file synchronization",
	Long: `MinusSync (msync) v` + cmd.Version + ` Build` + cmd.BuildNum + ` — a version control and file
synchronization system similar to git and lix. It supports code, markdown,
and binary files with features including version management, remote sync,
semantic search, and binary incremental sync.`,
	Version: cmd.Version + " Build" + cmd.BuildNum,
	Run: func(c *cobra.Command, args []string) {
		c.Help()
	},
}

func main() {
	cmd.RegisterCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
