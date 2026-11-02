package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/analysis"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/spf13/cobra"
)

var (
	testDiff         string
	testUseCoverage  string
	testPrintCommand bool
	testDepth        int
	testExclude      []string
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Identify affected tests",
	Long: `Map code changes to affected test cases. Helps you run only the tests
that matter instead of the full test suite.

Examples:
  # Find tests affected by last commit
  impact test --diff HEAD~1

  # Use coverage data for higher precision
  impact test --diff HEAD~1 --use-coverage ./coverage.out

  # Output as JSON for CI integration
  impact test --diff HEAD~1 --format json`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().StringVarP(&testDiff, "diff", "d", "", "git diff reference (e.g., HEAD~1, origin/master...HEAD)")
	testCmd.Flags().StringVar(&testUseCoverage, "use-coverage", "", "path to coverage file for higher precision")
	testCmd.Flags().BoolVar(&testPrintCommand, "print-command", false, "print executable test command instead of test list")
	testCmd.Flags().IntVar(&testDepth, "depth", 5, "maximum traversal depth (-1 for unlimited; default 5)")
	testCmd.Flags().StringSliceVar(&testExclude, "exclude", nil, "paths to exclude (glob patterns, merged with .impact.yaml)")

	testCmd.MarkFlagRequired("diff")
}

func runTest(cmd *cobra.Command, args []string) error {
	if err := requireAdapters(); err != nil {
		return err
	}

	orch, cleanup, err := newOrchestrator(testExclude)
	if err != nil {
		return err
	}
	defer cleanup()

	if testUseCoverage != "" {
		if _, err := os.Stat(testUseCoverage); err != nil {
			return fmt.Errorf("coverage file not found: %w", err)
		}
		verboseLog("Using coverage file: %s", testUseCoverage)
	}

	defer timedStep("test mapping")()

	result, err := orch.AnalyzeDiff(testDiff, effectiveDepth(cmd, testDepth, orch.Config()))
	if err != nil {
		return err
	}

	affectedTests := result.AffectedTests

	// Annotate tests with coverage status if coverage file is provided
	if testUseCoverage != "" {
		if err := annotateTestsWithCoverage(affectedTests, testUseCoverage); err != nil {
			verboseLog("Warning: failed to annotate with coverage: %v", err)
		}
	}

	if len(affectedTests) == 0 {
		fmt.Println("No affected tests found.")
		return nil
	}

	if testPrintCommand {
		// Print executable test command
		cmd, err := generateTestCommand(affectedTests)
		if err != nil {
			return fmt.Errorf("failed to generate test command: %w", err)
		}
		fmt.Println(cmd)
		return nil
	}

	switch strings.ToLower(format) {
	case "json":
		payload, err := json.MarshalIndent(affectedTests, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal affected tests: %w", err)
		}
		fmt.Println(string(payload))
	default:
		fmt.Println("Affected tests:")
		for _, test := range affectedTests {
			statusStr := ""
			if test.CoverageStatus != "" {
				statusStr = fmt.Sprintf(" [%s]", test.CoverageStatus)
			}
			fmt.Printf("- %s (%s:%d)%s\n", test.Name, test.File, test.Line, statusStr)
		}
	}

	return nil
}

// annotateTestsWithCoverage annotates tests with coverage status (annotation mode, not filtering)
func annotateTestsWithCoverage(tests []*model.Symbol, coveragePath string) error {
	// Load coverage profile
	profile, err := analysis.LoadCoverageProfile(coveragePath)
	if err != nil {
		return fmt.Errorf("failed to load coverage profile: %w", err)
	}

	// Annotate each test
	for _, test := range tests {
		if test != nil {
			analysis.AnnotateSymbol(test, profile)
		}
	}

	return nil
}

// filterTestsByCoverage is deprecated and kept for backwards compatibility
// It filters tests to only those covered by the coverage file
func filterTestsByCoverage(tests []*model.Symbol, coveragePath string) []*model.Symbol {
	data, err := os.ReadFile(coveragePath)
	if err != nil {
		verboseLog("failed to read coverage file: %v", err)
		return tests
	}

	coveredFiles := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		coveredFiles[filepath.Clean(parts[0])] = struct{}{}
	}

	if len(coveredFiles) == 0 {
		return tests
	}

	filtered := make([]*model.Symbol, 0, len(tests))
	for _, test := range tests {
		if test == nil {
			continue
		}
		testPath := filepath.Clean(test.File)

		if _, ok := coveredFiles[testPath]; ok {
			filtered = append(filtered, test)
			continue
		}

		if rel, err := filepath.Rel(".", testPath); err == nil {
			if _, ok := coveredFiles[filepath.Clean(rel)]; ok {
				filtered = append(filtered, test)
			}
		}
	}

	if len(filtered) == 0 {
		return tests
	}
	return filtered
}

// generateTestCommand generates an executable test command based on detected language
func generateTestCommand(tests []*model.Symbol) (string, error) {
	if len(tests) == 0 {
		return "", nil
	}

	// Group tests by language
	testsByLang := make(map[model.Language][]*model.Symbol)
	for _, test := range tests {
		testsByLang[test.Language] = append(testsByLang[test.Language], test)
	}

	// Generate command for the primary language (most tests)
	var primaryLang model.Language
	maxTests := 0
	for lang, langTests := range testsByLang {
		if len(langTests) > maxTests {
			maxTests = len(langTests)
			primaryLang = lang
		}
	}

	tests = testsByLang[primaryLang]

	switch primaryLang {
	case model.LanguageGo:
		// Group by package/directory
		packages := make(map[string][]string)
		for _, test := range tests {
			pkg := filepath.Dir(test.File)
			packages[pkg] = append(packages[pkg], test.Name)
		}

		// Build go test command
		var pkgPaths []string
		for pkg := range packages {
			pkgPaths = append(pkgPaths, "./"+pkg+"/...")
		}

		// Build test name pattern
		var testNames []string
		for _, pkgTests := range packages {
			testNames = append(testNames, pkgTests...)
		}

		if len(testNames) > 0 {
			pattern := strings.Join(testNames, "|")
			return fmt.Sprintf("go test -run '%s' %s", pattern, strings.Join(pkgPaths, " ")), nil
		}
		return fmt.Sprintf("go test %s", strings.Join(pkgPaths, " ")), nil

	case model.LanguageJava:
		// Group test classes
		var classNames []string
		seen := make(map[string]bool)
		for _, test := range tests {
			// Extract class name from file path
			base := filepath.Base(test.File)
			className := strings.TrimSuffix(base, filepath.Ext(base))
			if !seen[className] {
				classNames = append(classNames, className)
				seen[className] = true
			}
		}

		return fmt.Sprintf("mvn test -Dtest=%s", strings.Join(classNames, ",")), nil

	case model.LanguageJavaScript, model.LanguageTypeScript:
		// Group test files
		var testFiles []string
		seen := make(map[string]bool)
		for _, test := range tests {
			if !seen[test.File] {
				testFiles = append(testFiles, test.File)
				seen[test.File] = true
			}
		}

		return fmt.Sprintf("npx jest %s", strings.Join(testFiles, " ")), nil

	case model.LanguagePython:
		var testFiles []string
		seen := make(map[string]bool)
		for _, test := range tests {
			if !seen[test.File] {
				testFiles = append(testFiles, test.File)
				seen[test.File] = true
			}
		}
		if _, err := os.Stat("pytest.ini"); err == nil {
			return fmt.Sprintf("pytest %s", strings.Join(testFiles, " ")), nil
		}
		if _, err := os.Stat("pyproject.toml"); err == nil {
			return fmt.Sprintf("pytest %s", strings.Join(testFiles, " ")), nil
		}
		return fmt.Sprintf("python -m unittest %s", strings.Join(testFiles, " ")), nil

	default:
		return "", fmt.Errorf("unsupported language for test command generation: %s", primaryLang)
	}
}
