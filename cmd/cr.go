package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	crDiff     string
	crTemplate string
	crDepth    int
	crExclude  []string
)

var crCmd = &cobra.Command{
	Use:   "cr",
	Short: "Generate code review description",
	Long: `Generate structured impact analysis for PR/MR descriptions.
Auto-documents what changed, what's affected, and what to review.

Examples:
  # Generate CR description for last commit
  impact cr --diff HEAD~1

  # Generate CR description vs master
  impact cr --diff origin/master...HEAD

  # Use custom template
  impact cr --diff HEAD~1 --template ./.impact/cr-template.md`,
	RunE: runCR,
}

func init() {
	rootCmd.AddCommand(crCmd)

	crCmd.Flags().StringVarP(&crDiff, "diff", "d", "", "git diff reference (e.g., HEAD~1, origin/master...HEAD)")
	crCmd.Flags().StringVar(&crTemplate, "template", "", "path to custom Go template file")
	crCmd.Flags().IntVar(&crDepth, "depth", 5, "maximum traversal depth (-1 for unlimited; default 5)")
	crCmd.Flags().StringSliceVar(&crExclude, "exclude", nil, "paths to exclude (glob patterns, merged with .impact.yaml)")

	crCmd.MarkFlagRequired("diff")
}

func runCR(cmd *cobra.Command, args []string) error {
	if err := requireAdapters(); err != nil {
		return err
	}

	orch, cleanup, err := newOrchestrator(crExclude)
	if err != nil {
		return err
	}
	defer cleanup()

	defer timedStep("code review report")()

	result, err := orch.AnalyzeDiff(crDiff, effectiveDepth(cmd, crDepth, orch.Config()))
	if err != nil {
		return err
	}

	reporter, err := selectReporter("markdown", crTemplate)
	if err != nil {
		return err
	}

	output, err := reporter.Generate(result)
	if err != nil {
		return err
	}

	fmt.Println(output)

	return nil
}
