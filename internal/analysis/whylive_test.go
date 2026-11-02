package analysis

import (
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestWhyReachable_DirectEntry(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{
		Name:     "main",
		Kind:     model.SymbolKindFunction,
		File:     "main.go",
		Line:     1,
		Language: model.LanguageGo,
		Package:  "main",
	}
	cg.AddSymbol(entry)

	cfg := config.OrphanConfig{
		Heuristics: config.OrphanHeuristics{EnableInit: true},
	}
	analyzer := NewOrphanAnalyzer(cg, cfg, nil)

	result, err := analyzer.WhyReachable("main.main")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Reachable {
		t.Error("Expected main to be reachable")
	}
	if len(result.Path) != 0 {
		t.Error("Expected empty path for direct entry point")
	}
}

func TestWhyReachable_OneHop(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{
		Name:     "main",
		Kind:     model.SymbolKindFunction,
		File:     "main.go",
		Line:     1,
		Language: model.LanguageGo,
		Package:  "main",
	}
	handler := &model.Symbol{
		Name:     "handler",
		Kind:     model.SymbolKindFunction,
		File:     "handler.go",
		Line:     5,
		Language: model.LanguageGo,
		Package:  "main",
	}
	cg.AddSymbol(entry)
	cg.AddSymbol(handler)
	cg.AddEdge(entry, handler, model.EdgeKindCall)

	cfg := config.OrphanConfig{
		Heuristics: config.OrphanHeuristics{EnableInit: true},
	}
	analyzer := NewOrphanAnalyzer(cg, cfg, nil)

	result, err := analyzer.WhyReachable("main.handler")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Reachable {
		t.Error("Expected handler to be reachable")
	}
	if len(result.Path) != 1 {
		t.Errorf("Expected 1 hop in path, got %d", len(result.Path))
	}
	if len(result.Path) > 0 {
		if result.Path[0].From != "main.main" || result.Path[0].To != "main.handler" {
			t.Errorf("Unexpected path: from=%s to=%s", result.Path[0].From, result.Path[0].To)
		}
	}
}

func TestWhyReachable_NotReachable(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{
		Name:     "main",
		Kind:     model.SymbolKindFunction,
		File:     "main.go",
		Line:     1,
		Language: model.LanguageGo,
		Package:  "main",
	}
	orphan := &model.Symbol{
		Name:     "orphan",
		Kind:     model.SymbolKindFunction,
		File:     "orphan.go",
		Line:     10,
		Language: model.LanguageGo,
		Package:  "main",
	}
	cg.AddSymbol(entry)
	cg.AddSymbol(orphan)

	cfg := config.OrphanConfig{
		Heuristics: config.OrphanHeuristics{EnableInit: true},
	}
	analyzer := NewOrphanAnalyzer(cg, cfg, nil)

	result, err := analyzer.WhyReachable("main.orphan")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Reachable {
		t.Error("Expected orphan to be NOT reachable")
	}
}

func TestWhyReachable_NotFound(t *testing.T) {
	cg := graph.NewCallGraph()
	cfg := config.OrphanConfig{}
	analyzer := NewOrphanAnalyzer(cg, cfg, nil)

	_, err := analyzer.WhyReachable("nonexistent.Func")
	if err == nil {
		t.Error("Expected error for non-existent symbol")
	}
}

func TestFormatWhyReachablePath(t *testing.T) {
	tests := []struct {
		name     string
		result   *WhyReachableResult
		contains string
	}{
		{
			name: "not reachable",
			result: &WhyReachableResult{
				Target:    "pkg.Dead",
				Reachable: false,
			},
			contains: "NOT reachable",
		},
		{
			name: "entry point",
			result: &WhyReachableResult{
				Target:    "main.main",
				Reachable: true,
				Path:      nil,
			},
			contains: "entry point",
		},
		{
			name: "with path",
			result: &WhyReachableResult{
				Target:    "pkg.Handler",
				Reachable: true,
				Path: []PathEdge{
					{From: "main.main", To: "router.Setup", EdgeKind: "call"},
					{From: "router.Setup", To: "pkg.Handler", EdgeKind: "call"},
				},
			},
			contains: "2 hops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatWhyReachablePath(tt.result)
			if !strings.Contains(output, tt.contains) {
				t.Errorf("Expected output to contain %q, got: %s", tt.contains, output)
			}
		})
	}
}
