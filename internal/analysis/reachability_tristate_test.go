package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestTriState_TypeRefOnlyIsWeak(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	typeOnly := &model.Symbol{Name: "Config", Kind: model.SymbolKindStruct, Package: "pkg", File: "config.go"}

	cg.AddEdge(entry, typeOnly, model.EdgeKindTypeRef)
	cg.Finalize()

	entries := map[string]*model.Symbol{entry.QualifiedName(): entry}
	states := NewReachabilityMarker(cg).MarkTriState(entries)

	if states[entry.QualifiedName()] != StrongReach {
		t.Errorf("entry should be StrongReach, got %d", states[entry.QualifiedName()])
	}
	if states[typeOnly.QualifiedName()] != WeakReach {
		t.Errorf("type-ref-only symbol should be WeakReach, got %d", states[typeOnly.QualifiedName()])
	}
}

func TestTriState_CallEdgeIsStrong(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	called := &model.Symbol{Name: "Process", Kind: model.SymbolKindFunction, Package: "pkg", File: "svc.go"}

	cg.AddEdge(entry, called, model.EdgeKindCall)
	cg.Finalize()

	entries := map[string]*model.Symbol{entry.QualifiedName(): entry}
	states := NewReachabilityMarker(cg).MarkTriState(entries)

	if states[called.QualifiedName()] != StrongReach {
		t.Errorf("call-edge symbol should be StrongReach, got %d", states[called.QualifiedName()])
	}
}

func TestTriState_InheritanceIsStrong(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	impl := &model.Symbol{Name: "Handler", Kind: model.SymbolKindStruct, Package: "pkg", File: "handler.go"}

	cg.AddEdge(entry, impl, model.EdgeKindInheritance)
	cg.Finalize()

	entries := map[string]*model.Symbol{entry.QualifiedName(): entry}
	states := NewReachabilityMarker(cg).MarkTriState(entries)

	if states[impl.QualifiedName()] != StrongReach {
		t.Errorf("inheritance-edge symbol should be StrongReach, got %d", states[impl.QualifiedName()])
	}
}

func TestTriState_DecoratorIsWeak(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	decorated := &model.Symbol{Name: "myDecorator", Kind: model.SymbolKindFunction, Package: "pkg", File: "deco.go"}

	cg.AddEdge(entry, decorated, model.EdgeKindDecorator)
	cg.Finalize()

	entries := map[string]*model.Symbol{entry.QualifiedName(): entry}
	states := NewReachabilityMarker(cg).MarkTriState(entries)

	if states[decorated.QualifiedName()] != WeakReach {
		t.Errorf("decorator-edge symbol should be WeakReach, got %d", states[decorated.QualifiedName()])
	}
}

func TestTriState_UnreachableNotInMap(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	orphan := &model.Symbol{Name: "orphan", Package: "pkg", File: "orphan.go"}
	cg.AddSymbol(entry)
	cg.AddSymbol(orphan)
	cg.Finalize()

	entries := map[string]*model.Symbol{entry.QualifiedName(): entry}
	states := NewReachabilityMarker(cg).MarkTriState(entries)

	if _, exists := states[orphan.QualifiedName()]; exists {
		t.Errorf("orphan should not be in triState map at all")
	}
}

func TestTriState_MixedEdges(t *testing.T) {
	cg := graph.NewCallGraph()
	entry := &model.Symbol{Name: "main", Package: "pkg", File: "main.go"}
	callTarget := &model.Symbol{Name: "serve", Kind: model.SymbolKindFunction, Package: "pkg", File: "server.go"}
	typeTarget := &model.Symbol{Name: "Config", Kind: model.SymbolKindStruct, Package: "pkg", File: "config.go"}
	transitiveWeak := &model.Symbol{Name: "SubConfig", Kind: model.SymbolKindStruct, Package: "pkg", File: "sub.go"}

	cg.AddEdge(entry, callTarget, model.EdgeKindCall)
	cg.AddEdge(entry, typeTarget, model.EdgeKindTypeRef)
	cg.AddEdge(typeTarget, transitiveWeak, model.EdgeKindTypeRef)
	cg.Finalize()

	entries := map[string]*model.Symbol{entry.QualifiedName(): entry}
	states := NewReachabilityMarker(cg).MarkTriState(entries)

	if states[callTarget.QualifiedName()] != StrongReach {
		t.Errorf("call target should be StrongReach")
	}
	if states[typeTarget.QualifiedName()] != WeakReach {
		t.Errorf("type ref target should be WeakReach")
	}
	if states[transitiveWeak.QualifiedName()] != WeakReach {
		t.Errorf("transitive type ref should be WeakReach")
	}
}
