package analysis

import (
	"fmt"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestSSAConfirmedWeight(t *testing.T) {
	cfg := config.OrphanConfidence{
		SSAConfirmedWeight: 2.0,
		KindFunctionWeight: 1.0,
	}
	scorer := NewConfidenceScorer(cfg, nil)

	sym := &model.Symbol{
		Name:     "deadFunc",
		Kind:     model.SymbolKindFunction,
		File:     "test.go",
		Line:     10,
		Language: model.LanguageGo,
	}
	cg := graph.NewCallGraph()
	cg.AddSymbol(sym)

	// Without SSA confirmed
	candidate := scorer.Score(sym, cg, false, NoiseFlags{}, Unreachable)
	baseScore := candidate.Score

	// With SSA confirmed
	candidate2 := scorer.Score(sym, cg, false, NoiseFlags{SSAConfirmedOrphan: true}, Unreachable)
	if candidate2.Score != baseScore+2.0 {
		t.Errorf("Expected score %.1f with SSA confirmed, got %.1f", baseScore+2.0, candidate2.Score)
	}

	// Should have SSA confirmation reason
	found := false
	for _, reason := range candidate2.Reasons {
		if reason == "SSA/RTA confirms symbol is unreachable" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected SSA confirmation reason in candidate reasons")
	}
}

func TestOrphanAnalyzer_SSAPrimaryMode(t *testing.T) {
	// Create a call graph with a Go function that tree-sitter thinks is reachable
	// but SSA says is unreachable
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

	// A function that SSA says is NOT reachable
	unreachableFunc := &model.Symbol{
		Name:     "deadFunc",
		Kind:     model.SymbolKindFunction,
		File:     "dead.go",
		Line:     5,
		Language: model.LanguageGo,
		Package:  "main",
	}
	cg.AddSymbol(unreachableFunc)

	// A function that SSA says IS reachable
	reachableFunc := &model.Symbol{
		Name:     "liveFunc",
		Kind:     model.SymbolKindFunction,
		File:     "live.go",
		Line:     10,
		Language: model.LanguageGo,
		Package:  "main",
	}
	cg.AddSymbol(reachableFunc)

	// Add call edge: main -> reachableFunc (tree-sitter)
	cg.AddEdge(entry, reachableFunc, model.EdgeKindCall)

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			SSAConfirmedWeight: 2.0,
			KindFunctionWeight: 1.0,
		},
	}

	// Mock SSA reachability: only main and liveFunc are reachable
	mockReach := &mockSSAReachability{
		reachable: map[PosKey]bool{
			{File: "main.go", Line: 1, Name: "main"}:      true,
			{File: "live.go", Line: 10, Name: "liveFunc"}: true,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil)
	analyzer = analyzer.WithGoReachability(mockReach)

	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	// deadFunc should be in candidates (SSA confirmed orphan)
	found := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "deadFunc" {
			found = true
			// Should have SSA confirmed reason
			hasReason := false
			for _, r := range c.Reasons {
				if r == "SSA/RTA confirms symbol is unreachable" {
					hasReason = true
				}
			}
			if !hasReason {
				t.Error("Expected SSA confirmed reason for deadFunc")
			}
			break
		}
	}
	if !found {
		t.Error("Expected deadFunc to be in orphan candidates")
	}

	// liveFunc should NOT be in candidates (SSA says reachable)
	for _, c := range result.Candidates {
		if c.Symbol.Name == "liveFunc" {
			t.Error("liveFunc should not be in orphan candidates - SSA says it's reachable")
		}
	}
}

func TestOrphanAnalyzer_Diagnostics(t *testing.T) {
	cg := graph.NewCallGraph()
	cfg := config.OrphanConfig{}

	// Test with failing SSA provider
	failingReach := &mockSSAReachability{err: fmt.Errorf("no go.mod found")}
	analyzer := NewOrphanAnalyzer(cg, cfg, nil)
	analyzer = analyzer.WithGoReachability(failingReach)

	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	// Should have a warning diagnostic
	if len(result.Diagnostics) == 0 {
		t.Fatal("Expected diagnostics when SSA fails")
	}
	if result.Diagnostics[0].Level != "warning" {
		t.Errorf("Expected warning level, got %s", result.Diagnostics[0].Level)
	}
	if result.Diagnostics[0].Detail != "no go.mod found" {
		t.Errorf("Expected error detail, got %s", result.Diagnostics[0].Detail)
	}
}

type mockSSAReachability struct {
	reachable map[PosKey]bool
	err       error
}

func (m *mockSSAReachability) Reachable() (map[PosKey]bool, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.reachable, nil
}
