package graph

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestCallGraph_TypeRefGetCallers(t *testing.T) {
	cg := NewCallGraph()
	holder := &model.Symbol{Name: "Holder", Package: "pkg", File: "holder.go"}
	dep := &model.Symbol{Name: "Dep", Kind: model.SymbolKindStruct, Package: "pkg", File: "dep.go"}
	cg.AddEdge(holder, dep, model.EdgeKindTypeRef)
	cg.Finalize()

	callers := cg.GetCallers(dep)
	if len(callers) != 1 || callers[0].Name != "Holder" {
		t.Fatalf("GetCallers(type ref) = %#v, want Holder", callers)
	}

	typeRefCallers := cg.GetCallersByKind(dep, model.EdgeKindTypeRef)
	if len(typeRefCallers) != 1 || typeRefCallers[0].Name != "Holder" {
		t.Fatalf("GetCallersByKind(type_ref) = %#v, want Holder", typeRefCallers)
	}

	callCallers := cg.GetCallersByKind(dep, model.EdgeKindCall)
	if len(callCallers) != 0 {
		t.Fatalf("GetCallersByKind(call) = %#v, want none", callCallers)
	}
}
