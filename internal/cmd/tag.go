package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func tagCmd() *cobra.Command {
	var deleteFlag bool
	var annotated bool
	var message string

	cmd := &cobra.Command{
		Use:   "tag [-l] [-d <name>] [-a -m <msg>] [name] [ref]",
		Short: "List, create, or delete tags",
		Long:  "Manage tags in the repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			// Delete tag
			if deleteFlag {
				if len(args) < 1 {
					return fmt.Errorf("tag name required for delete")
				}
				return r.Refs.DeleteTag(args[0])
			}

			// Create tag
			if len(args) > 0 {
				name := args[0]
				ref := "HEAD"
				if len(args) > 1 {
					ref = args[1]
				}

				var targetHash, err = r.Refs.ResolveHEAD(r.HeadPath())
				if err != nil {
					return err
				}
				_ = ref // For now use HEAD

				if annotated {
					if message == "" {
						return fmt.Errorf("message required for annotated tag (-m)")
					}
					tagger := r.Config.GetDefault("user", "name", "unknown")
					email := r.Config.GetDefault("user", "email", "unknown@unknown")
					tagData := object.NewTagData(targetHash, object.TypeCommit, name, tagger, email, message)
					tagHash, err := object.WriteTag(r.ObjectsPath(), tagData)
					if err != nil {
						return err
					}
					return r.Refs.SetTag(name, tagHash)
				}
				return r.Refs.SetTag(name, targetHash)
			}

			// List tags
			tags, err := r.Refs.ListTags()
			if err != nil {
				return err
			}
			for _, t := range tags {
				fmt.Println(t)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteFlag, "delete", "d", false, "Delete a tag")
	cmd.Flags().BoolVarP(&annotated, "annotate", "a", false, "Create an annotated tag")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Tag message")
	return cmd
}
