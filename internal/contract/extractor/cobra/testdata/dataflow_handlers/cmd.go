package dataflow_handlers

import "github.com/spf13/cobra"

// newHandlersCmd tests dataflow to downstream handlers
func newHandlersCmd() *cobra.Command {
	var input string
	var output string

	cmd := &cobra.Command{
		Use:   "handlers",
		Short: "Handlers test",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Dataflow to handlers
			result := ProcessInput(input)
			WriteOutput(output, result)
			return nil
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "input file")
	cmd.Flags().StringVar(&output, "output", "", "output file")

	return cmd
}

// ProcessInput is a downstream handler
func ProcessInput(path string) string {
	return "processed: " + path
}

// WriteOutput is another downstream handler
func WriteOutput(path string, content string) {
	_ = path
	_ = content
}
