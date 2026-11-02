package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestGoAdapter_TypeRef_FieldType(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
type Holder struct {
	Dep Dep
}
`)
	assertTypeRefCall(t, calls, "Dep", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_AddressCompositeLiteral(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
func New() *Dep {
	return &Dep{}
}
`)
	assertTypeRefCall(t, calls, "Dep", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_CompositeLiteral(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
func New() Dep {
	return Dep{}
}
`)
	assertTypeRefCall(t, calls, "Dep", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_TypeAssertion(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Runner interface{ Run() }
func Use(v any) {
	_ = v.(Runner)
}
`)
	assertTypeRefCall(t, calls, "Runner", model.SymbolKindInterface)
}

func TestGoAdapter_TypeRef_ParametersAndReturns(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Input struct{}
type Output struct{}
func Transform(in Input) Output {
	return Output{}
}
`)
	assertTypeRefCall(t, calls, "Input", model.SymbolKindStruct)
	assertTypeRefCall(t, calls, "Output", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_PointerType(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
func Use(dep *Dep) {}
`)
	assertTypeRefCall(t, calls, "Dep", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_SelectorType(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
import "example.com/project/pkg"
type Holder struct {
	Dep *pkg.Dep
}
`)
	for _, call := range calls {
		if call.Callee != nil && call.Callee.Name == "Dep" && call.Callee.Package == "pkg" && call.EdgeKind == model.EdgeKindTypeRef {
			return
		}
	}
	t.Fatalf("selector type_ref call to pkg.Dep not found in %#v", calls)
}

// Test qualified type references (Commit C - #5)
func TestGoAdapter_TypeRef_QualifiedTypes(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
import (
	"context"
	"time"
)
type Service struct {
	When time.Time
	Ctx  context.Context
}
func Process(ctx context.Context, t time.Time) *time.Time {
	return &time.Time{}
}
`)
	// Should have multiple references to time.Time and context.Context
	var timeCount, contextCount int
	for _, call := range calls {
		if call.EdgeKind == model.EdgeKindTypeRef && call.Callee != nil {
			if call.Callee.Name == "Time" && call.Callee.Package == "time" {
				timeCount++
			}
			if call.Callee.Name == "Context" && call.Callee.Package == "context" {
				contextCount++
			}
		}
	}
	if timeCount < 2 {
		t.Errorf("Expected at least 2 time.Time references, got %d", timeCount)
	}
	if contextCount < 2 {
		t.Errorf("Expected at least 2 context.Context references, got %d", contextCount)
	}
}

func TestGoAdapter_TypeRef_VarDeclaredType(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
var x Dep
var y *Dep
const c Dep = Dep{}
`)
	assertTypeRefCall(t, calls, "Dep", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_ContainerTypes(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
var m map[string][]*Dep
var ch chan Dep
var arr [4]Dep
`)
	count := 0
	for _, call := range calls {
		if call.Callee != nil && call.Callee.Name == "Dep" && call.EdgeKind == model.EdgeKindTypeRef {
			count++
		}
	}
	if count < 3 {
		t.Fatalf("Expected at least 3 Dep type_ref calls (map value, chan, array), got %d: %#v", count, calls)
	}
}

func TestGoAdapter_TypeRef_GenericInstantiation(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Dep struct{}
type List[T any] struct{}
var l List[Dep]
`)
	assertTypeRefCall(t, calls, "List", model.SymbolKindStruct)
	assertTypeRefCall(t, calls, "Dep", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_InterfaceElements(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
type Base interface{}
type Res struct{}
type A interface {
	Base
	M() Res
}
`)
	assertTypeRefCall(t, calls, "Base", model.SymbolKindStruct)
	assertTypeRefCall(t, calls, "Res", model.SymbolKindStruct)
}

func TestGoAdapter_TypeRef_QualifiedNested(t *testing.T) {
	calls := extractTypeRefCalls(t, `package main
import "example.com/project/pkg"
var m map[string][]*pkg.Dep
var l pkg.List[pkg.Dep]
`)
	found := 0
	for _, call := range calls {
		if call.Callee != nil && call.Callee.Name == "Dep" && call.Callee.Package == "pkg" && call.EdgeKind == model.EdgeKindTypeRef {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("Expected at least 2 qualified pkg.Dep type_ref calls, got %d: %#v", found, calls)
	}
	for _, call := range calls {
		if call.Callee != nil && call.Callee.Name == "List" && call.Callee.Package == "pkg" && call.EdgeKind == model.EdgeKindTypeRef {
			return
		}
	}
	t.Fatalf("qualified generic type_ref pkg.List not found in %#v", calls)
}

func extractTypeRefCalls(t *testing.T, code string) []*model.CallSite {
	t.Helper()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "typeref.go")
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	calls, err := (&GoAdapter{}).ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}
	return calls
}

func assertTypeRefCall(t *testing.T, calls []*model.CallSite, name string, kind model.SymbolKind) {
	t.Helper()
	for _, call := range calls {
		if call.Callee != nil && call.Callee.Name == name && call.EdgeKind == model.EdgeKindTypeRef && call.Callee.Kind == kind {
			return
		}
	}
	t.Fatalf("type_ref call to %s (%s) not found in %#v", name, kind, calls)
}
