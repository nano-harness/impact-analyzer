package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/diff"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/report"
	"github.com/spf13/cobra"
)

var (
	surfaceDiff       string
	surfaceKinds      []string
	surfaceExtractors []string
	surfaceMaxTokens  int
)

var surfaceCmd = &cobra.Command{
	Use:   "surface",
	Short: "Diff public API surface (CLI commands, HTTP routes, RPC methods, etc.)",
	Long: `Detect changes in public contracts (CLI commands, HTTP routes, RPC methods,
exported functions, etc.) between git refs.

Public contracts are the "API surface" of your application - the entry points
and interfaces that external users or systems depend on. Changes to these
contracts can have broader implications than internal code changes.

Examples:
  # Diff CLI command surface
  impact surface --diff HEAD~1 --kind cli_command

  # Multi-kind analysis
  impact surface --diff origin/master...HEAD --kind cli_command,http_route

  # JSON output for CI integration
  impact surface --diff HEAD~1 --format json

  # LLM-friendly summary for AI tools
  impact surface --diff HEAD~1 --format llm-context --max-tokens 4000

  # Specific framework
  impact surface --diff HEAD~1 --extractor cobra`,
	RunE: runSurface,
}

func init() {
	rootCmd.AddCommand(surfaceCmd)

	surfaceCmd.Flags().StringVarP(&surfaceDiff, "diff", "d", "", "git diff reference (required, e.g., HEAD~1, origin/master...HEAD)")
	surfaceCmd.Flags().StringSliceVar(&surfaceKinds, "kind", nil, "filter by contract kind (cli_command,http_route,frontend_route,rpc_method,grpc_service,exported_symbol)")
	surfaceCmd.Flags().StringSliceVar(&surfaceExtractors, "extractor", nil, "filter by extractor name (cobra,gin,echo,grpc,express,commander,spring-mvc,nestjs,react-router,axum,clap)")
	surfaceCmd.Flags().IntVar(&surfaceMaxTokens, "max-tokens", 8000, "maximum tokens for llm-context format (0 = unlimited)")

	surfaceCmd.MarkFlagRequired("diff")
}

func runSurface(cmd *cobra.Command, args []string) error {
	if surfaceDiff == "" {
		return fmt.Errorf("--diff flag is required")
	}

	// Get repository root
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get repository root: %w", err)
	}

	// Get changed files from git
	gitOps := diff.NewGit(repoRoot)
	changedFiles, err := gitOps.ChangedFiles(surfaceDiff)
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		fmt.Println("No files changed in the specified diff.")
		return nil
	}

	verboseLog("Found %d changed files", len(changedFiles))

	// Select extractors based on flags
	extractors, err := selectExtractors()
	if err != nil {
		return err
	}

	if len(extractors) == 0 {
		return fmt.Errorf("no contract extractors available (try installing language-specific tools or check --kind/--extractor flags)")
	}

	verboseLog("Using %d contract extractors", len(extractors))
	if len(contract.MatchedExtractorNames(repoRoot, changedFiles, extractors)) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "[surface] no extractor matched any of %d changed files (tried: %s)\n", len(changedFiles), strings.Join(contract.ExtractorNames(extractors), ", "))
	}

	// Extract base ref from diff (e.g., HEAD~1 from "HEAD~1" or "origin/master" from "origin/master...HEAD")
	baseRef := parseBaseRef(surfaceDiff)
	verboseLog("Base ref: %s", baseRef)

	// Load contracts from old and new versions
	oldContracts, newContracts, err := contract.LoadContractsForDiff(repoRoot, baseRef, changedFiles, extractors)
	if err != nil {
		return fmt.Errorf("failed to load contracts: %w", err)
	}

	verboseLog("Old contracts: %d, New contracts: %d", len(oldContracts), len(newContracts))

	// Compute diff
	contractDiff, err := contract.ComputeDiff(oldContracts, newContracts)
	if err != nil {
		return fmt.Errorf("failed to compute contract diff: %w", err)
	}

	// Generate report
	reporter, err := selectSurfaceReporter(format, surfaceMaxTokens)
	if err != nil {
		return err
	}

	output, err := reporter.GenerateSurface(contractDiff)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	fmt.Println(output)

	return nil
}

// selectExtractors selects contract extractors based on --kind and --extractor flags
func selectExtractors() ([]contract.ContractExtractor, error) {
	var extractors []contract.ContractExtractor

	// If specific extractors requested, use those
	if len(surfaceExtractors) > 0 {
		for _, name := range surfaceExtractors {
			extractor, err := contract.Get(name)
			if err != nil {
				return nil, fmt.Errorf("extractor %q not found: %w", name, err)
			}
			extractors = append(extractors, extractor)
		}
		return extractors, nil
	}

	// If specific kinds requested, get extractors for those kinds
	if len(surfaceKinds) > 0 {
		kindSet := make(map[model.ContractKind]bool)
		for _, kindStr := range surfaceKinds {
			kindSet[model.ContractKind(kindStr)] = true
		}

		for _, extractor := range contract.All() {
			if kindSet[extractor.Kind()] {
				extractors = append(extractors, extractor)
			}
		}
		return extractors, nil
	}

	// No filters specified, use all registered extractors
	return contract.All(), nil
}

// parseBaseRef extracts the base ref from a diff specification
// Examples:
//   - "HEAD~1" -> "HEAD~1"
//   - "origin/master...HEAD" -> "origin/master"
//   - "abc123..def456" -> "abc123"
func parseBaseRef(diffSpec string) string {
	// Handle three-dot notation (symmetric difference)
	if strings.Contains(diffSpec, "...") {
		parts := strings.Split(diffSpec, "...")
		return parts[0]
	}

	// Handle two-dot notation (range)
	if strings.Contains(diffSpec, "..") {
		parts := strings.Split(diffSpec, "..")
		return parts[0]
	}

	// Single ref (e.g., HEAD~1)
	return diffSpec
}

// selectSurfaceReporter creates the appropriate surface reporter
func selectSurfaceReporter(format string, maxTokens int) (report.SurfaceReporter, error) {
	switch format {
	case "text":
		return &report.SurfaceTextReporter{}, nil
	case "json":
		return &report.SurfaceJSONReporter{}, nil
	case "llm-context":
		return &report.SurfaceLLMContextReporter{MaxTokens: maxTokens}, nil
	default:
		return nil, fmt.Errorf("unsupported format for surface command: %s (supported: text, json, llm-context)", format)
	}
}
