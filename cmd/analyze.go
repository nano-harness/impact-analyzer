package cmd

import (
	"fmt"
	"os"

	"github.com/nano-harness/impact-analyzer/internal/analysis"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/spf13/cobra"
)

var (
	analyzeDiff                string
	analyzeFile                string
	analyzeDepth               int
	analyzeExclude             []string
	analyzeNoSurface           bool
	analyzeUseCoverage         string
	analyzeTraceContracts      *bool
	analyzeNoTraceContracts    bool
	analyzeTraceContractsDepth int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze code change impact",
	Long: `Analyze the impact of code changes by building call graphs and identifying
affected callers, breaking changes, and risk levels.

Examples:
  # Analyze last commit
  impact analyze --diff HEAD~1

  # Analyze changes vs master branch
  impact analyze --diff origin/master...HEAD

  # Analyze all dependents of a file
  impact analyze --file path/to/file.go

  # Limit traversal depth
  impact analyze --diff HEAD~1 --depth 5`,
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVarP(&analyzeDiff, "diff", "d", "", "git diff reference (e.g., HEAD~1, origin/master...HEAD)")
	analyzeCmd.Flags().StringVar(&analyzeFile, "file", "", "analyze all dependents of a specific file")
	analyzeCmd.Flags().IntVar(&analyzeDepth, "depth", 5, "maximum traversal depth (-1 for unlimited; default 5 prevents caller explosion)")
	analyzeCmd.Flags().StringSliceVar(&analyzeExclude, "exclude", nil, "paths to exclude (glob patterns, merged with .impact.yaml)")
	analyzeCmd.Flags().BoolVar(&analyzeNoSurface, "no-surface-diff", false, "disable surface contract diff (faster, less complete)")
	analyzeCmd.Flags().StringVar(&analyzeUseCoverage, "use-coverage", "", "annotate callers with coverage status from a Go coverage profile")
	analyzeCmd.Flags().BoolVar(&analyzeNoTraceContracts, "no-trace-contracts", false, "disable contract tracing (method -> contracts)")
	analyzeCmd.Flags().IntVar(&analyzeTraceContractsDepth, "trace-depth", 8, "maximum depth for contract tracing")

	analyzeCmd.MarkFlagsMutuallyExclusive("diff", "file")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	if err := requireAdapters(); err != nil {
		return err
	}

	if analyzeDiff == "" && analyzeFile == "" {
		return fmt.Errorf("either --diff or --file must be specified")
	}

	orch, cleanup, err := newOrchestrator(analyzeExclude)
	if err != nil {
		return err
	}
	defer cleanup()

	// Apply CLI flag overrides to config
	if analyzeNoTraceContracts {
		orch.Config().Analysis.TraceAffectedContracts = false
	}
	if analyzeTraceContractsDepth > 0 && cmd.Flags().Changed("trace-depth") {
		orch.Config().Analysis.TraceMaxDepth = analyzeTraceContractsDepth
	}

	defer timedStep("analysis")()

	depth := effectiveDepth(cmd, analyzeDepth, orch.Config())

	var impactResult *model.ImpactResult
	if analyzeFile != "" {
		impactResult, err = orch.AnalyzeFiles([]string{analyzeFile}, depth)
	} else {
		// Use AnalyzeDiffWithSurface with flag control
		withSurface := !analyzeNoSurface
		impactResult, err = orch.AnalyzeDiffWithSurface(analyzeDiff, depth, withSurface)
	}
	if err != nil {
		return err
	}

	// Annotate callers with coverage status if coverage file is provided
	if analyzeUseCoverage != "" {
		if _, err := os.Stat(analyzeUseCoverage); err != nil {
			return fmt.Errorf("coverage file not found: %w", err)
		}
		verboseLog("Using coverage file: %s", analyzeUseCoverage)

		if err := annotateCallerswithCoverage(impactResult, analyzeUseCoverage); err != nil {
			verboseLog("Warning: failed to annotate with coverage: %v", err)
		}
	}

	reporter, err := selectReporter(format, "")
	if err != nil {
		return err
	}

	output, err := reporter.Generate(impactResult)
	if err != nil {
		return err
	}

	fmt.Println(output)
	verboseLog("Impact summary: %s", impactResult.Summary())

	// Diagnostic hint when no changes detected
	if len(impactResult.Changes) == 0 {
		verboseLog("Hint: No changes detected. This may indicate:")
		verboseLog("  - The diff is empty")
		verboseLog("  - Changed files use unsupported languages")
		verboseLog("  - Language adapters failed to parse changed files")
		verboseLog("Run with --verbose for more details")
	}

	return nil
}

// annotateCallerswithCoverage annotates all callers in the impact result with coverage status
func annotateCallerswithCoverage(result *model.ImpactResult, coveragePath string) error {
	// Load coverage profile
	profile, err := analysis.LoadCoverageProfile(coveragePath)
	if err != nil {
		return fmt.Errorf("failed to load coverage profile: %w", err)
	}

	// Annotate callers in each change
	for _, change := range result.Changes {
		if change == nil {
			continue
		}
		for _, caller := range change.AllCallers {
			if caller != nil {
				analysis.AnnotateSymbol(caller, profile)
			}
		}
	}

	return nil
}
