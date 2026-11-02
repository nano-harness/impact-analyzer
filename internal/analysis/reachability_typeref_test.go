package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestReachability_TypeRefEdges(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	holder := &model.Symbol{Name: "Holder", Kind: model.SymbolKindStruct, Package: "pkg", File: "holder.go"}
	dep := &model.Symbol{Name: "Dep", Kind: model.SymbolKindStruct, Package: "pkg", File: "dep.go"}
	cg.AddEdge(entry, holder)
	cg.AddEdge(holder, dep, model.EdgeKindTypeRef)
	cg.Finalize()

	reachable := NewReachabilityMarker(cg).Mark(map[string]*model.Symbol{entry.QualifiedName(): entry})
	if !reachable[dep.QualifiedName()] {
		t.Fatalf("type_ref callee %s should be reachable", dep.QualifiedName())
	}
}
