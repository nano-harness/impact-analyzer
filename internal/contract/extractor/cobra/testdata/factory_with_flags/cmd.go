package factory_with_flags

import "github.com/spf13/cobra"

func newCmd() *cobra.Command {
	var extraMvnArgs []string
	var useMvnScan bool
	cmd := &cobra.Command{
		Use:   "deploy <project-dir>",
		Short: "Deploy packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = extraMvnArgs
			_ = useMvnScan
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&extraMvnArgs, "mvn-args", nil, "Maven extra args")
	cmd.Flags().BoolVar(&useMvnScan, "use-mvn-scan", false, "Enable scan")
	return cmd
}
