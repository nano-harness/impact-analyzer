package graph

import (
	"github.com/nano-harness/impact-analyzer/internal/model"
	"testing"
)

// Baseline is the port before the QualifiedName/lookup-only optimization.
func addSymbolBeforeOptimization(cg *CallGraph, symbol *model.Symbol) {
	if symbol == nil {
		return
	}
	if existing := cg.symbols[symbol.QualifiedName()]; existing != nil && existing.File != "" && symbol.File == "" {
		return
	}
	cg.finalized = false
	cg.symbols[symbol.QualifiedName()] = symbol
	cg.symbolsByName[symbol.Name] = append(cg.symbolsByName[symbol.Name], symbol)
	if symbol.File != "" {
		byName := cg.symbolsByFile[symbol.File]
		if byName == nil {
			byName = make(map[string][]*model.Symbol)
			cg.symbolsByFile[symbol.File] = byName
		}
		byName[symbol.Name] = append(byName[symbol.Name], symbol)
	}
}

func BenchmarkAddSymbolIdentity(b *testing.B) {
	declaration := &model.Symbol{Name: "Read", Owner: "Box", Package: "example.com/identity", File: "box.go"}
	reference := &model.Symbol{Name: declaration.Name, Owner: declaration.Owner, Package: declaration.Package}
	for _, tc := range []struct {
		name string
		add  func(*CallGraph, *model.Symbol)
	}{
		{"before", addSymbolBeforeOptimization}, {"optimized", (*CallGraph).AddSymbol},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				cg := NewCallGraph()
				tc.add(cg, declaration)
				tc.add(cg, reference)
			}
		})
	}
}
