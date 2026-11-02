package reads_freevar

import "github.com/spf13/cobra"

// newFreeVarCmd tests FreeVar reads (Layer 1)
func newFreeVarCmd() *cobra.Command {
	var config string

	cmd := &cobra.Command{
		Use:   "freevar",
		Short: "FreeVar test",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Direct use - captured as FreeVar
			processConfig(config)
			return nil
		},
	}

	cmd.Flags().StringVar(&config, "config", "", "config file")

	return cmd
}

func processConfig(cfg string) {
	// Business logic
	_ = cfg
}
