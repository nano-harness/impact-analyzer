package pattern_a_closure

import "github.com/spf13/cobra"

// newClosureCmd creates a command with factory closure (Pattern A with closure)
func newClosureCmd() *cobra.Command {
	var flagName string

	makeRunE := func() func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			// Use flag in closure
			_ = flagName
			return nil
		}
	}

	cmd := &cobra.Command{
		Use:   "closure",
		Short: "Command with closure",
		RunE:  makeRunE(),
	}

	cmd.Flags().StringVar(&flagName, "name", "", "name flag")

	return cmd
}
