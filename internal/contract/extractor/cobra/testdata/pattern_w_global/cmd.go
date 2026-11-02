package pattern_w_global

import "github.com/spf13/cobra"

// Pattern W: Global variable cobra command
var globalCmd = &cobra.Command{
	Use:   "global",
	Short: "Global command",
	Long:  "This is a global command for testing Pattern W",
	RunE:  runGlobal,
}

var globalFlag string

func init() {
	globalCmd.Flags().StringVar(&globalFlag, "option", "", "option flag")
}

func runGlobal(cmd *cobra.Command, args []string) error {
	// Use global flag
	_ = globalFlag
	return nil
}
