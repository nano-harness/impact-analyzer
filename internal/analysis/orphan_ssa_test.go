package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// fakeGoReachability is a test implementation of GoReachabilityProvider
type fakeGoReachability struct {
	positions map[PosKey]bool
	err       error
}

func (f *fakeGoReachability) Reachable() (map[PosKey]bool, error) {
	return f.positions, f.err
}

// TestOrphanAnalyzer_WithSSAReachability verifies SSA reachability integration
func TestOrphanAnalyzer_WithSSAReachability(t *testing.T) {
	cg := graph.NewCallGraph()

	// Create some Go functions
	main := &model.Symbol{
		Name:     "main",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/main.go",
		Line:     10,
		Language: model.LanguageGo,
	}
	reachableFunc := &model.Symbol{
		Name:     "reachable",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/lib.go",
		Line:     20,
		Language: model.LanguageGo,
	}
	unreachableFunc := &model.Symbol{
		Name:     "unreachable",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/lib.go",
		Line:     30,
		Language: model.LanguageGo,
	}

	cg.AddSymbol(main)
	cg.AddSymbol(reachableFunc)
	cg.AddSymbol(unreachableFunc)
	// Only connect main to reachableFunc in tree-sitter graph
	cg.AddEdge(main, reachableFunc)
	cg.Finalize()

	// Create fake SSA provider that marks reachableFunc as reachable
	fakeSSA := &fakeGoReachability{
		positions: map[PosKey]bool{
			PosKeyOf("/repo/main.go", 10, "main"):     true, // main
			PosKeyOf("/repo/lib.go", 20, "reachable"): true, // reachableFunc
			// unreachableFunc at line 30 is NOT in SSA reachable set
		},
	}

	cfg := config.OrphanConfig{
		Heuristics: config.OrphanHeuristics{EnableInit: true},
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
			NewCodeWeight:      3,
			ExportedOffset:     -1.5,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil).WithGoReachability(fakeSSA)
	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	// Verify that unreachableFunc is in candidates
	foundUnreachable := false
	foundReachable := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "unreachable" {
			foundUnreachable = true
		}
		if c.Symbol.Name == "reachable" {
			foundReachable = true
		}
	}

	if !foundUnreachable {
		t.Error("Expected unreachableFunc to be in orphan candidates (not reachable by SSA)")
	}
	if foundReachable {
		t.Error("reachableFunc should not be in candidates (marked reachable by SSA)")
	}
}

// TestOrphanAnalyzer_SSAReachabilityError verifies fallback when SSA fails
func TestOrphanAnalyzer_SSAReachabilityError(t *testing.T) {
	cg := graph.NewCallGraph()

	orphan := &model.Symbol{
		Name:     "orphan",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/lib.go",
		Line:     10,
		Language: model.LanguageGo,
	}
	cg.AddSymbol(orphan)
	cg.Finalize()

	// Create fake SSA provider that returns error
	fakeSSA := &fakeGoReachability{
		err: nil, // Will return (nil, nil) which triggers fallback
	}

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil).WithGoReachability(fakeSSA)
	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	// Should still work with tree-sitter only
	foundOrphan := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "orphan" {
			foundOrphan = true
		}
	}

	if !foundOrphan {
		t.Error("Expected orphan to be found even when SSA fails")
	}
}

// TestOrphanAnalyzer_SSAReachabilityOnlyForGoFunctions verifies language/kind filtering
func TestOrphanAnalyzer_SSAReachabilityOnlyForGoFunctions(t *testing.T) {
	cg := graph.NewCallGraph()

	// Create non-Go symbol
	jsFunc := &model.Symbol{
		Name:     "jsFunc",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/app.js",
		Line:     10,
		Language: model.LanguageJavaScript,
	}

	// Create Go type (not function/method)
	goType := &model.Symbol{
		Name:     "MyStruct",
		Kind:     model.SymbolKindStruct,
		File:     "/repo/types.go",
		Line:     20,
		Language: model.LanguageGo,
	}

	cg.AddSymbol(jsFunc)
	cg.AddSymbol(goType)
	cg.Finalize()

	// Create SSA provider with empty set
	fakeSSA := &fakeGoReachability{
		positions: map[PosKey]bool{},
	}

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil).WithGoReachability(fakeSSA)
	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	// Both should be in candidates because SSA reachability only applies to Go functions/methods
	foundJS := false
	foundType := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "jsFunc" {
			foundJS = true
		}
		if c.Symbol.Name == "MyStruct" {
			foundType = true
		}
	}

	if !foundJS {
		t.Error("JavaScript function should be in candidates (SSA doesn't apply)")
	}
	if !foundType {
		t.Error("Go struct should be in candidates (SSA doesn't apply to types)")
	}
}

// TestOrphanAnalyzer_WithoutSSAReachability verifies backward compatibility
func TestOrphanAnalyzer_WithoutSSAReachability(t *testing.T) {
	cg := graph.NewCallGraph()

	orphan := &model.Symbol{
		Name:     "orphan",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/lib.go",
		Line:     10,
		Language: model.LanguageGo,
	}
	cg.AddSymbol(orphan)
	cg.Finalize()

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
		},
	}

	// Create analyzer WITHOUT SSA reachability
	analyzer := NewOrphanAnalyzer(cg, cfg, nil)
	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	// Should work exactly as before
	foundOrphan := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "orphan" {
			foundOrphan = true
		}
	}

	if !foundOrphan {
		t.Error("Expected orphan to be found with tree-sitter only")
	}
}
