package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestConfidence_HighGate(t *testing.T) {
	cfg := config.OrphanConfidence{
		KindFunctionWeight: 3,
		LargeBodyWeight:    1,
	}
	scorer := NewConfidenceScorer(cfg, nil, 5)
	cg := graph.NewCallGraph()
	target := &model.Symbol{Name: "Target", Kind: model.SymbolKindFunction, File: "target.go", Line: 1, EndLine: 20, IsExported: true, Language: model.LanguageGo}
	for i := 0; i < 6; i++ {
		cg.AddEdge(&model.Symbol{Name: "Caller" + string(rune('A'+i)), Package: "pkg"}, target)
	}
	cg.Finalize()

	candidate := scorer.Score(target, cg, false)
	if candidate.Confidence != model.ConfidenceMedium {
		t.Fatalf("confidence = %s, score %.1f; want medium after high gate", candidate.Confidence, candidate.Score)
	}
}
