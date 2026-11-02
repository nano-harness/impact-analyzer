package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nano-harness/impact-analyzer/internal/config"
	langpkg "github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/orchestrator"
	"github.com/nano-harness/impact-analyzer/internal/report"
	"github.com/spf13/cobra"
)

func newOrchestrator(cliExcludes []string) (*orchestrator.Orchestrator, func(), error) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config, using defaults: %v\n", err)
		cfg = nil // Orchestrator will use defaults
	}

	// Get absolute path to repository root (align with surface.go:67)
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return nil, func() {}, fmt.Errorf("failed to resolve repository root: %w", err)
	}

	// Resolve index/cache dir with priority: CLI flag > env (INDEX_CACHE_DIR) > config (index.cache_dir) > default.
	orch, err := orchestrator.NewOrchestratorWithOptions(repoRoot, resolveIndexDir(cfg), cfg, cliExcludes,
		orchestrator.WithVersion(GetVersion()))
	if err != nil {
		return nil, func() {}, err
	}

	cleanup := func() {
		if err := orch.Close(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "failed to close orchestrator: %v\n", err)
		}
	}
	return orch, cleanup, nil
}

func selectReporter(format string, templatePath string) (report.Reporter, error) {
	switch strings.ToLower(format) {
	case "text":
		return report.NewTextReporter(), nil
	case "json":
		return report.NewJSONReporter(), nil
	case "markdown", "md":
		return report.NewMarkdownReporter(templatePath)
	case "mermaid":
		return report.NewMermaidReporter(), nil
	case "llm-context", "llm":
		// Use global maxTokens variable from root.go
		return report.NewLLMContextReporter(maxTokens), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func verboseLog(msg string, args ...interface{}) {
	if !verbose {
		return
	}
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

// resolveIndexDir returns the effective index/cache directory with priority:
// --index-dir flag > INDEX_CACHE_DIR env > index.cache_dir config > .impact/cache.
func resolveIndexDir(cfg *config.Config) string {
	if indexDir != "" {
		return indexDir
	}
	if envDir := os.Getenv("INDEX_CACHE_DIR"); envDir != "" {
		return envDir
	}
	if cfg != nil && cfg.Index.CacheDir != "" {
		return cfg.Index.CacheDir
	}
	return ".impact/cache"
}

// effectiveDepth resolves the traversal depth: an explicit --depth flag wins,
// otherwise a positive analysis.max_depth from config overrides the flag default.
func effectiveDepth(cmd *cobra.Command, flagDepth int, cfg *config.Config) int {
	if cmd != nil && cmd.Flags().Changed("depth") {
		return flagDepth
	}
	if cfg != nil && cfg.Analysis.MaxDepth > 0 {
		return cfg.Analysis.MaxDepth
	}
	return flagDepth
}

func timedStep(label string) func() {
	start := time.Now()
	return func() {
		verboseLog("%s completed in %s", label, time.Since(start).Round(time.Millisecond))
	}
}

func requireAdapters() error {
	if len(langpkg.Available()) == 0 {
		return fmt.Errorf("no language adapters registered; this is a build issue (reinstall impact)")
	}
	if lang != "" && !langpkg.IsRegistered(model.Language(lang)) {
		return fmt.Errorf("no adapter for language %q; available: %v", lang, langpkg.Available())
	}
	return nil
}
