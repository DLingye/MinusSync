// Package cmd provides CLI command implementations for msync.
package cmd

import (
	"github.com/spf13/cobra"
)

// RegisterCommands registers all subcommands on the root command.
func RegisterCommands(root *cobra.Command) {
	root.AddCommand(initCmd())
	root.AddCommand(addCmd())
	root.AddCommand(commitCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(checkCmd())
	root.AddCommand(syncCmd())
	root.AddCommand(logCmd())
	root.AddCommand(diffCmd())
	root.AddCommand(tagCmd())
	root.AddCommand(remoteCmd())
	root.AddCommand(pushCmd())
	root.AddCommand(pullCmd())
	root.AddCommand(fetchCmd())
	root.AddCommand(cloneCmd())
	root.AddCommand(searchCmd())
	root.AddCommand(gcCmd())
	root.AddCommand(fsckCmd())
	root.AddCommand(configCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(versionCmd())
}
