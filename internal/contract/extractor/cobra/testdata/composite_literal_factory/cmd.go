package composite_literal_factory

import "github.com/spf13/cobra"

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "deploy <project-dir>",
		Short:   "Deploy packages",
		Long:    "Deploy Maven packages from a local project",
		Aliases: []string{"d", "dep"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
