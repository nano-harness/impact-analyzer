package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestOrphanAnalyzerFindsUnreachableSymbol(t *testing.T) {
	cg := graph.NewCallGraph()
	main := &model.Symbol{Name: "main", File: "main.go", Language: model.LanguageGo}
	wired := &model.Symbol{Name: "wired", File: "main.go", Language: model.LanguageGo}
	orphan := &model.Symbol{Name: "orphan", File: "feature.go", Language: model.LanguageGo}
	cg.AddSymbol(main)
	cg.AddSymbol(wired)
	cg.AddSymbol(orphan)
	cg.AddEdge(main, wired)
	cg.Finalize()
	cfg := config.OrphanConfig{Heuristics: config.OrphanHeuristics{EnableInit: true}, Confidence: config.OrphanConfidence{FileIsolatedWeight: 2, NewCodeWeight: 3, ExportedOffset: -1.5}}
	result := NewOrphanAnalyzer(cg, cfg, nil).Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})
	for _, c := range result.Candidates {
		if c.Symbol.Name == "orphan" {
			return
		}
	}
	t.Fatalf("expected orphan candidate, got %#v", result.Candidates)
}

func TestConfidence_KindWeight(t *testing.T) {
	cfg := config.OrphanConfidence{
		FileIsolatedWeight:        2,
		KindFunctionWeight:        1,
		KindVariableOffset:        -1.5,
		KindUnexportedConstOffset: -1,
		UnexportedOffset:          -1,
		LargeBodyWeight:           0.5,
		OnlyTestCallersOffset:     -1,
	}
	scorer := NewConfidenceScorer(cfg, nil)
	cg := graph.NewCallGraph()
	exportedFunc := &model.Symbol{Name: "PublicFunc", Kind: model.SymbolKindFunction, File: "service.go", Line: 1, EndLine: 12, IsExported: true, Language: model.LanguageGo}
	cg.AddSymbol(exportedFunc)
	candidate := scorer.Score(exportedFunc, cg, false)
	if candidate.Confidence != model.ConfidenceHigh {
		t.Fatalf("exported large function confidence = %s, score %.1f; want high", candidate.Confidence, candidate.Score)
	}

	localVar := &model.Symbol{Name: "local", Kind: model.SymbolKindVariable, File: "service.go", Line: 1, EndLine: 1, Language: model.LanguageGo}
	cg = graph.NewCallGraph()
	cg.AddSymbol(localVar)
	candidate = scorer.Score(localVar, cg, false)
	if candidate.Confidence != model.ConfidenceLow {
		t.Fatalf("local variable confidence = %s, score %.1f; want low", candidate.Confidence, candidate.Score)
	}

	callee := &model.Symbol{Name: "PublicCallee", Kind: model.SymbolKindFunction, File: "service.go", Line: 1, EndLine: 5, IsExported: true, Language: model.LanguageGo}
	testCaller := &model.Symbol{Name: "TestCaller", Kind: model.SymbolKindFunction, File: "service_test.go", IsTest: true, Language: model.LanguageGo}
	cg = graph.NewCallGraph()
	cg.AddEdge(testCaller, callee)
	candidate = scorer.Score(callee, cg, onlyTestCallers(cg.GetCallers(callee)))
	if candidate.Score >= 2 {
		t.Fatalf("only-test callers score = %.1f; want below medium", candidate.Score)
	}
}

func TestOrphan_SoftNoise(t *testing.T) {
	cg := graph.NewCallGraph()
	var shortFunc, longFunc *model.Symbol
	for _, sym := range []*model.Symbol{
		{Name: "localVar", Kind: model.SymbolKindVariable, File: "service.go", Line: 10, EndLine: 10, Column: 4, Language: model.LanguageGo},
		{Name: "packageVar", Kind: model.SymbolKindVariable, File: "service.go", Line: 1, EndLine: 1, Column: 1, Language: model.LanguageGo},
		{Name: "privateConst", Kind: model.SymbolKindConstant, File: "service.go", Line: 2, EndLine: 2, Column: 1, Language: model.LanguageGo},
		{Name: "tinyStruct", Kind: model.SymbolKindStruct, File: "service.go", Line: 3, EndLine: 4, Column: 1, Language: model.LanguageGo},
		{Name: "shortFunc", Kind: model.SymbolKindFunction, File: "service.go", Line: 5, EndLine: 6, Column: 1, Language: model.LanguageGo},
		{Name: "longFunc", Kind: model.SymbolKindFunction, File: "service.go", Line: 7, EndLine: 12, Column: 1, Language: model.LanguageGo},
	} {
		cg.AddSymbol(sym)
		if sym.Name == "shortFunc" {
			shortFunc = sym
		}
		if sym.Name == "longFunc" {
			longFunc = sym
		}
	}
	cg.AddEdge(longFunc, shortFunc)
	cfg := config.OrphanConfig{
		NoiseFilter: config.OrphanNoiseFilter{
			SkipLocalVariables:      true,
			SkipUnexportedConstants: true,
			SkipTinyStructs:         true,
			SkipShortFunctions:      true,
		},
		Confidence: config.OrphanConfidence{
			FileIsolatedWeight:     2,
			KindFunctionWeight:     1,
			UnexportedOffset:       -1,
			LocalVariablePenalty:   -0.8,
			UnexportedConstPenalty: -0.6,
			TinyStructPenalty:      -0.5,
			ShortFunctionPenalty:   -0.4,
		},
	}
	result := NewOrphanAnalyzer(cg, cfg, nil).Analyze(OrphanOptions{MinConfidence: model.ConfidenceLow})
	candidates := map[string]*model.OrphanCandidate{}
	for _, candidate := range result.Candidates {
		candidates[candidate.Symbol.Name] = candidate
	}
	for _, name := range []string{"localVar", "privateConst", "tinyStruct", "shortFunc", "packageVar", "longFunc"} {
		if candidates[name] == nil {
			t.Fatalf("%s should remain in orphan candidates", name)
		}
	}
	if candidates["shortFunc"].Confidence != model.ConfidenceLow {
		t.Fatalf("shortFunc confidence = %s, want low", candidates["shortFunc"].Confidence)
	}
}
