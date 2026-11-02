package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/analysis"
	"github.com/nano-harness/impact-analyzer/internal/diff"
	"github.com/nano-harness/impact-analyzer/internal/orchestrator"
	"github.com/nano-harness/impact-analyzer/internal/report"
	"github.com/spf13/cobra"
)

var (
	orphanDiff           string
	orphanMinConfidence  string
	orphanIncludeTests   bool
	orphanExclude        []string
	orphanLimit          int
	orphanAll            bool
	orphanGroupBy        string
	orphanOnlyNewIslands bool
	orphanWhyReachable   string
)

var orphanCmd = &cobra.Command{
	Use:   "orphan",
	Short: "Detect code not reachable from known entry points",
	Long: `Detect orphan code by scanning the repository and finding symbols that are not
reachable from framework entries, schedulers, known main/init functions, or configured entries.`,
	RunE: runOrphan,
}

func init() {
	rootCmd.AddCommand(orphanCmd)
	orphanCmd.Flags().StringVar(&orphanDiff, "diff", "", "optional git diff reference used to raise confidence for changed symbols")
	orphanCmd.Flags().StringVar(&orphanMinConfidence, "min-confidence", "medium", "minimum confidence to report (high, medium, low)")
	orphanCmd.Flags().BoolVar(&orphanIncludeTests, "include-tests", false, "include test symbols and allow test entries")
	orphanCmd.Flags().StringSliceVar(&orphanExclude, "exclude", nil, "paths to exclude (glob patterns, merged with .impact.yaml)")
	orphanCmd.Flags().IntVar(&orphanLimit, "limit", 50, "maximum candidates to show in markdown/text output (0 is equivalent to --all)")
	orphanCmd.Flags().BoolVar(&orphanAll, "all", false, "show all candidates in markdown/text output")
	orphanCmd.Flags().StringVar(&orphanGroupBy, "group-by", "kind", "group markdown/text output by kind, dir, or none")
	orphanCmd.Flags().BoolVar(&orphanOnlyNewIslands, "only-new-islands", false, "only report orphans within changed files/ranges (CI gate mode)")
	orphanCmd.Flags().StringVar(&orphanWhyReachable, "why-reachable", "", "trace why a symbol is reachable (symbol name or qualified name)")
}

func runOrphan(cmd *cobra.Command, args []string) error {
	if err := requireAdapters(); err != nil {
		return err
	}
	orch, cleanup, err := newOrchestrator(orphanExclude)
	if err != nil {
		return err
	}
	defer cleanup()
	orch.Config().Orphan.Heuristics.EnableTestEntry = orphanIncludeTests

	// Handle --why-reachable mode
	if orphanWhyReachable != "" {
		return runWhyReachable(orch)
	}

	// Determine diff reference: explicit --diff flag or implicit from --only-new-islands with args
	diffRef := orphanDiff
	if diffRef == "" && orphanOnlyNewIslands && len(args) > 0 {
		diffRef = args[0]
	}

	changedFiles, err := orphanChangedFiles(diffRef)
	if err != nil {
		return err
	}

	// Build changed ranges from unified diff if available
	var changedRanges map[string][][2]int
	if diffRef != "" {
		changedRanges, err = orphanChangedRanges(diffRef)
		if err != nil {
			// Non-fatal: fall back to file-level filtering
			changedRanges = nil
		}
	}

	defer timedStep("orphan analysis")()
	result, err := orch.AnalyzeOrphans(analysis.OrphanOptions{
		MinConfidence:  analysis.ConfidenceFromString(orphanMinConfidence),
		ChangedFiles:   changedFiles,
		ChangedRanges:  changedRanges,
		IncludeTests:   orphanIncludeTests,
		OnlyNewIslands: orphanOnlyNewIslands,
	})
	if err != nil {
		return err
	}
	output, err := report.GenerateOrphanReportWithOptions(result, format, report.OrphanReportOptions{
		Limit:   orphanLimit,
		All:     orphanAll || orphanLimit == 0,
		GroupBy: orphanGroupBy,
	})
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

func runWhyReachable(orch *orchestrator.Orchestrator) error {
	defer timedStep("why-reachable")()
	result, err := orch.WhyReachable(orphanWhyReachable)
	if err != nil {
		return err
	}

	switch strings.ToLower(format) {
	case "json":
		b, jsonErr := json.MarshalIndent(result, "", "  ")
		if jsonErr != nil {
			return jsonErr
		}
		fmt.Println(string(b))
	default:
		fmt.Println(analysis.FormatWhyReachablePath(result))
	}
	return nil
}

func orphanChangedFiles(diffRef string) ([]string, error) {
	if diffRef == "" {
		return nil, nil
	}
	gitOps := diff.NewGit(".")
	return gitOps.ChangedFiles(diffRef)
}

func orphanChangedRanges(diffRef string) (map[string][][2]int, error) {
	if diffRef == "" {
		return nil, nil
	}
	gitOps := diff.NewGit(".")
	diffText, err := gitOps.Diff(diffRef)
	if err != nil {
		return nil, err
	}
	fileDiffs, err := diff.ParseUnifiedDiff(diffText)
	if err != nil {
		return nil, err
	}
	ranges := make(map[string][][2]int, len(fileDiffs))
	for _, fd := range fileDiffs {
		path := fd.NewPath
		if path == "" {
			path = fd.OldPath
		}
		for _, hunk := range fd.Hunks {
			start := hunk.NewStart
			end := hunk.NewStart + hunk.NewLines - 1
			if end < start {
				end = start
			}
			ranges[path] = append(ranges[path], [2]int{start, end})
		}
	}
	return ranges, nil
}
