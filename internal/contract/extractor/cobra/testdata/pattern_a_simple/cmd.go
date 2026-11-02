package pattern_a_simple

import "github.com/spf13/cobra"

// newSimpleCmd creates a simple cobra command (Pattern A: factory function)
func newSimpleCmd() *cobra.Command {
	var flagStr string
	var flagInt int

	cmd := &cobra.Command{
		Use:   "simple",
		Short: "A simple command",
		Long:  "This is a simple command for testing",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use the flags
			_ = flagStr
			_ = flagInt
			return nil
		},
	}

	cmd.Flags().StringVar(&flagStr, "text", "", "text flag")
	cmd.Flags().IntVar(&flagInt, "count", 0, "count flag")

	return cmd
}
