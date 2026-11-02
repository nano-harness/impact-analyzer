package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type fixedEntryChecker map[string]bool

func (c fixedEntryChecker) IsEntryPoint(sym *model.Symbol) bool {
	return sym != nil && c[sym.QualifiedName()]
}

func TestImpactAnalyzerCollectsCallersDepthDedupAndCycles(t *testing.T) {
	a := testSymbol("pkg", "A")
	b := testSymbol("pkg", "B")
	c := testSymbol("pkg", "C")
	d := testSymbol("pkg", "D")

	cg := graph.NewCallGraph()
	cg.AddEdge(a, b)
	cg.AddEdge(b, c)
	cg.AddEdge(c, d)
	cg.AddEdge(c, b) // cycle B <-> C should not loop forever
	cg.Finalize()

	result, err := NewImpactAnalyzer(cg).Analyze([]*model.Change{{Type: model.ChangeTypeModified, Symbol: d}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Changes[0].Reach; got != 3 {
		t.Fatalf("reach=%d, want 3 callers A/B/C", got)
	}

	limited, err := NewImpactAnalyzer(cg).Analyze([]*model.Change{{Type: model.ChangeTypeModified, Symbol: d}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := limited.Changes[0].Reach; got != 1 {
		t.Fatalf("depth-limited reach=%d, want direct caller only", got)
	}
}

func TestImpactAnalyzerDedupesDiamondCallers(t *testing.T) {
	a := testSymbol("pkg", "A")
	b := testSymbol("pkg", "B")
	c := testSymbol("pkg", "C")
	d := testSymbol("pkg", "D")

	cg := graph.NewCallGraph()
	cg.AddEdge(a, b)
	cg.AddEdge(a, c)
	cg.AddEdge(b, d)
	cg.AddEdge(c, d)
	cg.Finalize()

	result, err := NewImpactAnalyzer(cg).Analyze([]*model.Change{{Type: model.ChangeTypeModified, Symbol: d}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Changes[0].Reach; got != 3 {
		t.Fatalf("reach=%d, want A/B/C once each", got)
	}
}

func TestImpactAnalyzerLeafChangeHasNoCallers(t *testing.T) {
	leaf := testSymbol("pkg", "Leaf")
	cg := graph.NewCallGraph()
	cg.AddSymbol(leaf)

	result, err := NewImpactAnalyzer(cg).Analyze([]*model.Change{{Type: model.ChangeTypeModified, Symbol: leaf}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Changes[0].Reach; got != 0 {
		t.Fatalf("leaf reach=%d, want 0 to avoid false affected symbols", got)
	}
}

func TestImpactAnalyzerStopsAtEntryPoints(t *testing.T) {
	entry := testSymbol("pkg", "Handler")
	mid := testSymbol("pkg", "Service")
	leaf := testSymbol("pkg", "Leaf")
	root := testSymbol("pkg", "Root")

	cg := graph.NewCallGraph()
	cg.AddEdge(root, entry)
	cg.AddEdge(entry, mid)
	cg.AddEdge(mid, leaf)
	cg.Finalize()

	open, err := NewImpactAnalyzer(cg).Analyze([]*model.Change{{Type: model.ChangeTypeModified, Symbol: leaf}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := NewImpactAnalyzerWithEntryChecker(cg, fixedEntryChecker{entry.QualifiedName(): true}).Analyze([]*model.Change{{Type: model.ChangeTypeModified, Symbol: leaf}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Changes[0].Reach > open.Changes[0].Reach {
		t.Fatalf("stop-at-entry reach=%d exceeds open reach=%d", stopped.Changes[0].Reach, open.Changes[0].Reach)
	}
	if got := stopped.Changes[0].Reach; got != 2 {
		t.Fatalf("stop-at-entry reach=%d, want mid and entry included but not root", got)
	}
}

func testSymbol(pkg, name string) *model.Symbol {
	return &model.Symbol{Name: name, Package: pkg, Kind: model.SymbolKindFunction, File: pkg + "/" + name + ".go", Line: 1, EndLine: 3, Language: model.LanguageGo}
}
