package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/analysis"
	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func BenchmarkAnalyzeDiff(b *testing.B) {
	const diffRef = "HEAD~1"

	if err := exec.Command("git", "rev-parse", diffRef).Run(); err != nil {
		b.Skipf("git history not available for %s: %v", diffRef, err)
	}

	orch, err := NewOrchestrator(".", "")
	if err != nil {
		b.Fatalf("failed to create orchestrator: %v", err)
	}
	defer orch.Close()

	// Disable the result cache so each iteration measures the real analysis
	// instead of a cache hit.
	orch.cache = nil

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset the global contract index cache for the same reason:
		// contract enrichment must rebuild the index every iteration.
		globalContractIndexCache = nil
		if _, err := orch.AnalyzeDiff(diffRef, 2); err != nil {
			b.Fatalf("AnalyzeDiff failed: %v", err)
		}
	}
}

func BenchmarkParseFile(b *testing.B) {
	adapter, err := lang.Get(model.LanguageGo)
	if err != nil {
		b.Fatalf("failed to get Go adapter: %v", err)
	}

	target := filepath.Join(".", "main.go")
	if _, err := os.Stat(target); err != nil {
		b.Skipf("main.go not found: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := adapter.ParseFile(target); err != nil {
			b.Fatalf("ParseFile failed: %v", err)
		}
	}
}

func BenchmarkBuildCallGraph(b *testing.B) {
	orch, err := NewOrchestrator(".", "")
	if err != nil {
		b.Fatalf("failed to create orchestrator: %v", err)
	}
	defer orch.Close()

	files, err := orch.listSupportedFiles()
	if err != nil || len(files) == 0 {
		b.Skip("no source files available to build call graph")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orch.resetCallGraph()
		// Clear the per-run caches so every iteration measures a cold
		// parse+build, comparable to runs before the caches existed.
		orch.parseCache.Clear()
		orch.resetGoDeep()
		if err := orch.buildCallGraph(files); err != nil {
			b.Fatalf("buildCallGraph failed: %v", err)
		}
	}
}

func BenchmarkRiskClassification(b *testing.B) {
	classifier := analysis.NewRiskClassifier()
	impact := &model.ChangeImpact{
		Change: &model.Change{
			Type: model.ChangeTypeModified,
			Symbol: &model.Symbol{
				Name:       "BenchmarkTarget",
				Kind:       model.SymbolKindFunction,
				File:       "benchmark.go",
				Line:       10,
				IsExported: true,
			},
		},
		Reach:         12,
		DirectCallers: []*model.Symbol{{Name: "caller1"}, {Name: "caller2"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.ClassifyRisk(impact)
	}
}
