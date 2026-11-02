package python

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestAdapterParseFileCallsAndExports(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app.py")
	code := `__all__ = ["public"]
import os
from .services import worker
class Service:
    def method(self):
        helper()

def public():
    worker.run()

def _private():
    pass
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	adapter := &Adapter{}
	fs, err := adapter.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if fs.Language != model.LanguagePython {
		t.Fatalf("language = %s", fs.Language)
	}
	if !containsString(fs.Imports, "os") {
		t.Fatalf("expected import os, got %#v", fs.Imports)
	}
	if !containsString(fs.Imports, ".services") {
		t.Fatalf("expected import .services, got %#v", fs.Imports)
	}

	seenPublic := false
	seenPrivate := false
	for _, sym := range fs.Symbols {
		if sym.Name == "public" && sym.IsExported {
			seenPublic = true
		}
		if sym.Name == "_private" && !sym.IsExported {
			seenPrivate = true
		}
	}
	if !seenPublic {
		t.Fatalf("expected public to be exported: %#v", fs.Symbols)
	}
	if !seenPrivate {
		t.Fatalf("expected _private to be private: %#v", fs.Symbols)
	}

	calls, err := adapter.ExtractCalls(file)
	if err != nil {
		t.Fatal(err)
	}
	if !containsCall(calls, "helper") {
		t.Fatalf("expected helper call, got %#v", calls)
	}
	if !containsCall(calls, "run") {
		t.Fatalf("expected worker.run call, got %#v", calls)
	}
}

func TestAdapterIsTestFile(t *testing.T) {
	adapter := &Adapter{}
	if !adapter.IsTestFile("tests/test_app.py") {
		t.Fatal("expected tests/test_app.py to be a test file")
	}
	if !adapter.IsTestFile("conftest.py") {
		t.Fatal("expected conftest.py to be a test file")
	}
	if adapter.IsTestFile("app.py") {
		t.Fatal("expected app.py not to be a test file")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsCall(calls []*model.CallSite, want string) bool {
	for _, call := range calls {
		if call != nil && call.Callee != nil && call.Callee.Name == want {
			return true
		}
	}
	return false
}

func TestPythonAdapterIsTestNonTestSymbolInTestFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_helper.py")
	code := `class Helper:
    pass

def helper():
    pass
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := (&Adapter{}).ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Helper", "helper"} {
		sym := findPythonSymbol(fs.Symbols, name)
		if sym == nil {
			t.Fatalf("%s not found in symbols: %#v", name, fs.Symbols)
		}
		if !sym.IsTest {
			t.Fatalf("%s should be marked as test in a test_*.py file", name)
		}
	}
}

func findPythonSymbol(symbols []*model.Symbol, name string) *model.Symbol {
	for _, sym := range symbols {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

func TestPythonDecoratorExtraction(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "decorated.py")
	code := `@property
def name(self):
    return self._name

@staticmethod
def create():
    pass

@app.route("/api")
def handler():
    pass
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	adapter := &Adapter{}
	calls, err := adapter.ExtractCalls(file)
	if err != nil {
		t.Fatal(err)
	}

	decorators := 0
	for _, call := range calls {
		if call != nil && call.EdgeKind == model.EdgeKindDecorator {
			decorators++
		}
	}

	if decorators != 3 {
		t.Fatalf("expected 3 decorators, got %d. Calls: %#v", decorators, calls)
	}
}

func TestPythonModuleNameResolution(t *testing.T) {
	dir := t.TempDir()

	// Create a package structure: mypackage/submodule/module.py
	packageDir := filepath.Join(dir, "mypackage")
	submoduleDir := filepath.Join(packageDir, "submodule")
	if err := os.MkdirAll(submoduleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create __init__.py files
	if err := os.WriteFile(filepath.Join(packageDir, "__init__.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(submoduleDir, "__init__.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a module file
	modulePath := filepath.Join(submoduleDir, "module.py")
	if err := os.WriteFile(modulePath, []byte("def test(): pass"), 0644); err != nil {
		t.Fatal(err)
	}

	adapter := &Adapter{}
	fs, err := adapter.ParseFile(modulePath)
	if err != nil {
		t.Fatal(err)
	}

	expectedPkg := "mypackage.submodule.module"
	if fs.Package != expectedPkg {
		t.Fatalf("expected package %s, got %s", expectedPkg, fs.Package)
	}
}

// Test receiver owner hints on method calls: callee.Owner carries the
// best-effort receiver variable/attribute name (not a resolved type).
func TestPythonAdapter_ExtractCalls_ReceiverOwnerHint(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "svc.py")
	code := `class Service:
    def method(self):
        self.helper()
        self.repo.find(1)
        worker.run()
        get_svc().load()
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	adapter := &Adapter{}
	calls, err := adapter.ExtractCalls(file)
	if err != nil {
		t.Fatal(err)
	}

	// owners maps "name:line" -> callee.Owner
	owners := make(map[string]string)
	for _, call := range calls {
		if call == nil || call.Callee == nil || call.EdgeKind != "" {
			continue
		}
		owners[call.Callee.Name+":"+strconv.Itoa(call.Line)] = call.Callee.Owner
	}

	want := map[string]string{
		"helper:3": "",       // self.helper() -> self resolves to enclosing class, no hint
		"find:4":   "repo",   // self.repo.find(1) -> last chain segment
		"run:5":    "worker", // worker.run()
		"load:6":   "",       // get_svc().load() -> complex receiver, no hint
	}
	for key, wantOwner := range want {
		got, ok := owners[key]
		if !ok {
			t.Errorf("no call extracted for %s (all calls: %#v)", key, calls)
			continue
		}
		if got != wantOwner {
			t.Errorf("callee %s Owner = %q, want %q", key, got, wantOwner)
		}
	}
}
func TestPythonInheritanceEdges(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "models.py")
	code := `class Base:
    pass

class Foo(Base):
    pass

class Bar(module.Base):
    pass

class Skipped(object):
    pass

class Generic(list[int]):
    pass

class Meta(Base, metaclass=type):
    pass
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	calls, err := (&Adapter{}).ExtractCalls(file)
	if err != nil {
		t.Fatal(err)
	}
	var edges []*model.CallSite
	for _, call := range calls {
		if call != nil && call.EdgeKind == model.EdgeKindInheritance {
			edges = append(edges, call)
		}
	}
	names := map[string]bool{}
	for _, edge := range edges {
		names[edge.Callee.Name] = true
		if edge.Callee.Kind != model.SymbolKindClass {
			t.Fatalf("inheritance callee kind = %s, want class", edge.Callee.Kind)
		}
	}
	if !names["Base"] {
		t.Fatalf("expected inheritance edge to Base, got %#v", edges)
	}
	if !names["module.Base"] {
		t.Fatalf("expected inheritance edge to module.Base, got %#v", edges)
	}
	if names["object"] {
		t.Fatal("object base must be skipped")
	}
	if names["type"] {
		t.Fatal("metaclass keyword argument must not be an inheritance edge")
	}
}

func TestPythonModuleLevelCallAssignment(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app.py")
	code := `from fastapi import FastAPI, APIRouter

app = FastAPI()
router = APIRouter()
count = 42

def make():
    local = FastAPI()
    return local
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := (&Adapter{}).ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app", "router"} {
		sym := findPythonSymbol(fs.Symbols, name)
		if sym == nil {
			t.Fatalf("%s not found in symbols: %#v", name, fs.Symbols)
		}
		if sym.Kind != model.SymbolKindVariable {
			t.Fatalf("%s kind = %s, want variable", name, sym.Kind)
		}
		if !sym.IsExported {
			t.Fatalf("%s should be exported", name)
		}
	}
	if sym := findPythonSymbol(fs.Symbols, "count"); sym != nil {
		t.Fatal("non-call assignment must not be captured")
	}
	if sym := findPythonSymbol(fs.Symbols, "local"); sym != nil {
		t.Fatal("assignment inside a function must not be captured")
	}
}
