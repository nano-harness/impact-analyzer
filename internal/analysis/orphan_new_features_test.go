package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestOnlyNewIslands_FiltersToChangedRange(t *testing.T) {
	cg := graph.NewCallGraph()

	// Orphan in changed range
	orphanInRange := &model.Symbol{
		Name:     "newFunc",
		Kind:     model.SymbolKindFunction,
		File:     "feature/new.go",
		Line:     15,
		Package:  "feature",
		Language: model.LanguageGo,
	}
	// Orphan outside changed range
	orphanOutside := &model.Symbol{
		Name:     "oldFunc",
		Kind:     model.SymbolKindFunction,
		File:     "feature/old.go",
		Line:     5,
		Package:  "feature",
		Language: model.LanguageGo,
	}

	cg.AddSymbol(orphanInRange)
	cg.AddSymbol(orphanOutside)
	cg.Finalize()

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
			KindFunctionWeight: 1,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil)
	result := analyzer.Analyze(OrphanOptions{
		MinConfidence:  model.ConfidenceLow,
		OnlyNewIslands: true,
		ChangedFiles:   []string{"feature/new.go", "feature/old.go"},
		ChangedRanges: map[string][][2]int{
			"feature/new.go": {{10, 20}}, // only lines 10-20
		},
	})

	// Only orphanInRange should be in results
	foundInRange := false
	foundOutside := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "newFunc" {
			foundInRange = true
		}
		if c.Symbol.Name == "oldFunc" {
			foundOutside = true
		}
	}

	if !foundInRange {
		t.Error("Expected newFunc (in changed range) to be reported")
	}
	if foundOutside {
		t.Error("oldFunc (outside changed range) should NOT be reported")
	}
}

func TestOnlyNewIslands_FallbackToChangedFiles(t *testing.T) {
	cg := graph.NewCallGraph()

	inFile := &model.Symbol{
		Name:     "inFile",
		Kind:     model.SymbolKindFunction,
		File:     "feature/changed.go",
		Line:     5,
		Package:  "feature",
		Language: model.LanguageGo,
	}
	notInFile := &model.Symbol{
		Name:     "notInFile",
		Kind:     model.SymbolKindFunction,
		File:     "other/untouched.go",
		Line:     5,
		Package:  "other",
		Language: model.LanguageGo,
	}

	cg.AddSymbol(inFile)
	cg.AddSymbol(notInFile)
	cg.Finalize()

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
			KindFunctionWeight: 1,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil)
	result := analyzer.Analyze(OrphanOptions{
		MinConfidence:  model.ConfidenceLow,
		OnlyNewIslands: true,
		ChangedFiles:   []string{"feature/changed.go"},
		// No ChangedRanges → falls back to file-level
	})

	foundInFile := false
	foundOther := false
	for _, c := range result.Candidates {
		if c.Symbol.Name == "inFile" {
			foundInFile = true
		}
		if c.Symbol.Name == "notInFile" {
			foundOther = true
		}
	}

	if !foundInFile {
		t.Error("Expected inFile (in changed file) to be reported")
	}
	if foundOther {
		t.Error("notInFile (not in changed file) should NOT be reported")
	}
}

func TestCluster_GroupsByPackageAndFile(t *testing.T) {
	cg := graph.NewCallGraph()

	s1 := &model.Symbol{Name: "FuncA", Kind: model.SymbolKindFunction, File: "pkg/service.go", Line: 10, Package: "pkg", Language: model.LanguageGo}
	s2 := &model.Symbol{Name: "FuncB", Kind: model.SymbolKindFunction, File: "pkg/service.go", Line: 20, Package: "pkg", Language: model.LanguageGo}
	s3 := &model.Symbol{Name: "FuncC", Kind: model.SymbolKindFunction, File: "other/handler.go", Line: 5, Package: "other", Language: model.LanguageGo}

	cg.AddSymbol(s1)
	cg.AddSymbol(s2)
	cg.AddSymbol(s3)
	cg.Finalize()

	cfg := config.OrphanConfig{
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
			KindFunctionWeight: 1,
		},
	}

	analyzer := NewOrphanAnalyzer(cg, cfg, nil)
	result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

	if len(result.Clusters) == 0 {
		t.Fatal("Expected at least one cluster")
	}

	// Should have 2 clusters: one for pkg/service.go and one for other/handler.go
	if len(result.Clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(result.Clusters))
	}

	// First cluster should be the largest (2 items in pkg)
	if result.Clusters[0].Count != 2 {
		t.Errorf("Expected first cluster count=2, got %d", result.Clusters[0].Count)
	}
	if result.Clusters[0].Package != "pkg" {
		t.Errorf("Expected first cluster package=pkg, got %s", result.Clusters[0].Package)
	}
}

func TestConfidence_WeakReachAppliesOffset(t *testing.T) {
	cfg := config.OrphanConfidence{
		FileIsolatedWeight: 2,
		KindFunctionWeight: 1,
		WeakReachOffset:    -2.0,
	}
	scorer := NewConfidenceScorer(cfg, nil)
	cg := graph.NewCallGraph()
	sym := &model.Symbol{Name: "WeakFunc", Kind: model.SymbolKindFunction, File: "svc.go", Line: 1, EndLine: 2, Language: model.LanguageGo}
	cg.AddSymbol(sym)

	// Score without weak reach
	candidateNormal := scorer.Score(sym, cg, false)

	// Score with weak reach
	candidateWeak := scorer.Score(sym, cg, false, WeakReach)

	if candidateWeak.Score >= candidateNormal.Score {
		t.Errorf("WeakReach score (%.1f) should be less than normal score (%.1f)", candidateWeak.Score, candidateNormal.Score)
	}

	expectedDiff := -2.0
	actualDiff := candidateWeak.Score - candidateNormal.Score
	if actualDiff != expectedDiff {
		t.Errorf("Expected score difference of %.1f, got %.1f", expectedDiff, actualDiff)
	}
}
