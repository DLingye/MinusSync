package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func remoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote <add|remove|list|set-url> [args...]",
		Short: "Manage remote repositories",
		Long:  "Manage the set of remote repositories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}

	cmd.AddCommand(remoteAddCmd())
	cmd.AddCommand(remoteRemoveCmd())
	cmd.AddCommand(remoteListCmd())

	return cmd
}

func remoteAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a remote repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			name, url := args[0], args[1]
			section := fmt.Sprintf("remote \"%s\"", name)
			r.Config.Set(section, "url", url)
			return r.SaveConfig()
		},
	}
}

func remoteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			name := args[0]
			section := fmt.Sprintf("remote \"%s\"", name)
			r.Config.RemoveSection(section)
			r.Refs.DeleteRemoteRef(name)
			return r.SaveConfig()
		},
	}
}

func remoteListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List remote repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			for _, section := range r.Config.Sections() {
				if len(section) > 8 && section[:8] == "remote \"" {
					name := section[8 : len(section)-1]
					url := r.Config.Get(section, "url")
					fmt.Printf("%s\t%s\n", name, url)
				}
			}
			return nil
		},
	}
}
