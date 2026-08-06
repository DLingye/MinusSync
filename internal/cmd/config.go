package cmd

import (
	"fmt"
	"strings"

	"github.com/MinusSync/internal/repo"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	var global bool
	var unset bool
	var list bool

	cmd := &cobra.Command{
		Use:   "config [--global] [--list] [--unset] <key> [value]",
		Short: "Get and set repository configuration",
		Long:  "Get and set configuration options for the repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.Open(".")
			if err != nil && !global {
				return err
			}

			if list {
				if r == nil {
					return fmt.Errorf("not in a repository")
				}
				for _, section := range r.Config.Sections() {
					for _, key := range r.Config.Keys(section) {
						value := r.Config.Get(section, key)
						fmt.Printf("%s.%s=%s\n", section, key, value)
					}
				}
				return nil
			}

			if unset {
				if len(args) < 1 {
					return fmt.Errorf("key required for --unset")
				}
				section, key := parseConfigKey(args[0])
				r.Config.Unset(section, key)
				return r.SaveConfig()
			}

			if len(args) == 1 {
				// Get value
				if r == nil {
					return fmt.Errorf("not in a repository")
				}
				section, key := parseConfigKey(args[0])
				value := r.Config.Get(section, key)
				if value != "" {
					fmt.Println(value)
				}
				return nil
			}

			if len(args) >= 2 {
				// Set value
				if r == nil {
					return fmt.Errorf("not in a repository")
				}
				section, key := parseConfigKey(args[0])
				value := strings.Join(args[1:], " ")
				r.Config.Set(section, key, value)
				return r.SaveConfig()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Use global config")
	cmd.Flags().BoolVar(&unset, "unset", false, "Remove a config key")
	cmd.Flags().BoolVar(&list, "list", false, "List all config")
	return cmd
}

// parseConfigKey parses "section.key" or just "key" (defaults to "core" section).
func parseConfigKey(fullKey string) (section, key string) {
	parts := strings.SplitN(fullKey, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "core", parts[0]
}
