package cmd

import (
	"fmt"

	"github.com/MinusSync/internal/fsck"
	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func fsckCmd() *cobra.Command {
	var verbose bool
	var fix bool

	cmd := &cobra.Command{
		Use:   "fsck",
		Short: "Verify repository integrity",
		Long:  "Check the integrity of the repository's object store and references.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil {
				return err
			}
			defer r.Close()

			report, err := fsck.Verify(r.ObjectsPath(), verbose)
			if err != nil {
				return err
			}

			fmt.Printf("Checked %d objects\n", report.ObjectsChecked)

			if len(report.Errors) > 0 {
				fmt.Printf("\n%d errors found:\n", len(report.Errors))
				for _, e := range report.Errors {
					fmt.Printf("  [%s] %s: %s\n", e.Severity, e.Object, e.Message)
				}
			}

			if len(report.Warnings) > 0 {
				fmt.Printf("\n%d warnings:\n", len(report.Warnings))
				for _, w := range report.Warnings {
					fmt.Printf("  %s: %s\n", w.Object, w.Message)
				}
			}

			if len(report.Errors) == 0 && len(report.Warnings) == 0 {
				fmt.Println("Repository integrity check passed.")
			}

			if fix && len(report.Errors) > 0 {
				fmt.Println("Auto-fix not yet implemented.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")
	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to fix issues")
	return cmd
}
