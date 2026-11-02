package two_factories

import "github.com/spf13/cobra"

func newFirstCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "first",
		Short: "First command",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = name
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "name flag")
	return cmd
}

func newSecondCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "second",
		Short: "Second command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
