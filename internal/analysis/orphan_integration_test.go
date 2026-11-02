package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// TestOrphanIntegration_EndToEnd exercises the full orphan pipeline with all new features.
func TestOrphanIntegration_EndToEnd(t *testing.T) {
	cg := graph.NewCallGraph()

	// Entry point
	main := &model.Symbol{
		Name:     "main",
		Kind:     model.SymbolKindFunction,
		File:     "/repo/cmd/main.go",
		Line:     10,
		Package:  "main",
		Language: model.LanguageGo,
	}

	// Reachable via call
	handler := &model.Symbol{
		Name:       "HandleRequest",
		Kind:       model.SymbolKindMethod,
		File:       "/repo/internal/server/handler.go",
		Line:       25,
		EndLine:    50,
		Package:    "server",
		IsExported: true,
		Language:   model.LanguageGo,
	}

	// Only type-ref'd struct — should be WeakReach and reported as orphan candidate
	configStruct := &model.Symbol{
		Name:       "AppConfig",
		Kind:       model.SymbolKindStruct,
		File:       "/repo/internal/config/types.go",
		Line:       5,
		EndLine:    20,
		Package:    "config",
		IsExported: true,
		Language:   model.LanguageGo,
	}

	// Function only reached via type_ref (declared but call path not connected)
	zombieFunc := &model.Symbol{
		Name:       "ProcessConfig",
		Kind:       model.SymbolKindFunction,
		File:       "/repo/internal/config/process.go",
		Line:       10,
		EndLine:    30,
		Package:    "config",
		IsExported: true,
		Language:   model.LanguageGo,
	}

	// Completely unreachable function in a new file
	newOrphan := &model.Symbol{
		Name:       "NewFeature",
		Kind:       model.SymbolKindFunction,
		File:       "/repo/internal/feature/new.go",
		Line:       15,
		EndLine:    40,
		Package:    "feature",
		IsExported: true,
		Language:   model.LanguageGo,
	}

	// Build graph
	cg.AddSymbol(main)
	cg.AddSymbol(handler)
	cg.AddSymbol(configStruct)
	cg.AddSymbol(zombieFunc)
	cg.AddSymbol(newOrphan)

	// main -> handler (call)
	cg.AddEdge(main, handler, model.EdgeKindCall)
	// handler -> configStruct (type_ref only)
	cg.AddEdge(handler, configStruct, model.EdgeKindTypeRef)
	// configStruct -> zombieFunc (type_ref only)
	cg.AddEdge(configStruct, zombieFunc, model.EdgeKindTypeRef)

	cg.Finalize()

	cfg := config.OrphanConfig{
		Heuristics: config.OrphanHeuristics{EnableInit: true},
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight: 2,
			NewCodeWeight:      3,
			KindFunctionWeight: 1,
			WeakReachOffset:    -2.0,
		},
	}

	t.Run("FullScan_WeakReachReported", func(t *testing.T) {
		analyzer := NewOrphanAnalyzer(cg, cfg, nil)
		result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

		// handler should NOT be in candidates (StrongReach)
		for _, c := range result.Candidates {
			if c.Symbol.Name == "HandleRequest" {
				t.Error("HandleRequest has call edge from entry, should not be orphan")
			}
		}

		// newOrphan should be in candidates (completely unreachable)
		foundNew := false
		for _, c := range result.Candidates {
			if c.Symbol.Name == "NewFeature" {
				foundNew = true
			}
		}
		if !foundNew {
			t.Error("NewFeature should be an orphan candidate")
		}

		// Clusters should be present
		if len(result.Clusters) == 0 {
			t.Error("Expected clusters in result")
		}
	})

	t.Run("OnlyNewIslands_FiltersCorrectly", func(t *testing.T) {
		analyzer := NewOrphanAnalyzer(cg, cfg, nil)
		result := analyzer.Analyze(OrphanOptions{
			MinConfidence:  model.ConfidenceLow,
			OnlyNewIslands: true,
			ChangedFiles:   []string{"/repo/internal/feature/new.go"},
			ChangedRanges: map[string][][2]int{
				"/repo/internal/feature/new.go": {{10, 45}},
			},
		})

		// Only newOrphan should be reported
		if len(result.Candidates) == 0 {
			t.Fatal("Expected at least one candidate in --only-new-islands mode")
		}
		for _, c := range result.Candidates {
			if c.Symbol.File != "/repo/internal/feature/new.go" {
				t.Errorf("Unexpected candidate from file %s in only-new-islands mode", c.Symbol.File)
			}
		}
	})

	t.Run("PosKey_NoCrossContamination", func(t *testing.T) {
		// Verify that same-line symbols with different names don't collide in SSA
		key1 := PosKeyOf("/repo/file.go", 10, "funcA")
		key2 := PosKeyOf("/repo/file.go", 10, "funcB")

		fakeSSA := &fakeGoReachability{
			positions: map[PosKey]bool{
				key1: true,
			},
		}

		localCG := graph.NewCallGraph()
		symA := &model.Symbol{Name: "funcA", Kind: model.SymbolKindFunction, File: "/repo/file.go", Line: 10, Language: model.LanguageGo}
		symB := &model.Symbol{Name: "funcB", Kind: model.SymbolKindFunction, File: "/repo/file.go", Line: 10, Language: model.LanguageGo}
		localCG.AddSymbol(symA)
		localCG.AddSymbol(symB)
		localCG.Finalize()

		localAnalyzer := NewOrphanAnalyzer(localCG, cfg, nil).WithGoReachability(fakeSSA)
		result := localAnalyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

		// funcA should NOT be a candidate (SSA says reachable)
		// funcB SHOULD be a candidate (different name, not in SSA set)
		foundA := false
		foundB := false
		for _, c := range result.Candidates {
			if c.Symbol.Name == "funcA" {
				foundA = true
			}
			if c.Symbol.Name == "funcB" {
				foundB = true
			}
		}
		if foundA {
			t.Error("funcA is SSA-reachable, should not be reported")
		}
		if !foundB {
			t.Error("funcB is NOT SSA-reachable (different name), should be reported")
		}

		// key1 and key2 are distinct due to different Name fields
		if key1 == key2 {
			t.Error("PosKeys with different names should not be equal")
		}
	})

	t.Run("Clusters_HaveCorrectData", func(t *testing.T) {
		analyzer := NewOrphanAnalyzer(cg, cfg, nil)
		result := analyzer.Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})

		for _, cluster := range result.Clusters {
			if cluster.Package == "" {
				t.Error("Cluster should have a package")
			}
			if cluster.Count == 0 {
				t.Error("Cluster count should be > 0")
			}
			if len(cluster.Symbols) != cluster.Count {
				t.Errorf("Cluster symbols length (%d) != count (%d)", len(cluster.Symbols), cluster.Count)
			}
			if cluster.Confidence == "" {
				t.Error("Cluster should have confidence")
			}
		}
	})
}
