package bindings_complex

import "github.com/spf13/cobra"

// newComplexBindingsCmd tests multiple binding types
func newComplexBindingsCmd() *cobra.Command {
	var strFlag string
	var intFlag int
	var boolFlag bool
	var strSliceFlag []string

	cmd := &cobra.Command{
		Use:   "complex",
		Short: "Complex bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = strFlag
			_ = intFlag
			_ = boolFlag
			_ = strSliceFlag
			return nil
		},
	}

	// Multiple binding types
	cmd.Flags().StringVar(&strFlag, "str", "", "string flag")
	cmd.Flags().IntVar(&intFlag, "int", 0, "int flag")
	cmd.Flags().BoolVar(&boolFlag, "bool", false, "bool flag")
	cmd.Flags().StringSliceVar(&strSliceFlag, "slice", nil, "slice flag")

	// Persistent flags
	cmd.PersistentFlags().StringVar(&strFlag, "persistent", "", "persistent flag")

	return cmd
}
