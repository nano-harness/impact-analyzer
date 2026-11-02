package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestRustAdapterParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.rs")
	source := `use std::fmt; pub struct User; trait T{} impl User { pub fn id(&self){} } pub fn helper(){ id(); }`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := (&RustAdapter{}).ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fs.Language != model.LanguageRust {
		t.Fatalf("language = %s", fs.Language)
	}
	if len(fs.Imports) == 0 {
		t.Fatal("expected imports")
	}
	foundRaw := false
	for _, imp := range fs.Imports {
		if imp == "std::fmt" {
			foundRaw = true
		}
	}
	if !foundRaw {
		t.Fatalf("imports = %#v, want raw std::fmt path", fs.Imports)
	}
	kinds := map[model.SymbolKind]bool{}
	var method *model.Symbol
	for _, s := range fs.Symbols {
		kinds[s.Kind] = true
		if s.Kind == model.SymbolKindMethod {
			method = s
		}
	}
	for _, kind := range []model.SymbolKind{model.SymbolKindStruct, model.SymbolKindInterface, model.SymbolKindFunction, model.SymbolKindMethod} {
		if !kinds[kind] {
			t.Fatalf("missing kind %s in %#v", kind, kinds)
		}
	}
	if method == nil || method.Owner != "User" {
		t.Fatalf("impl method owner = %#v, want User", method)
	}
}

func TestRustAdapterExtractCalls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.rs")
	if err := os.WriteFile(path, []byte(`fn a(){ b(); c.d(); crate::e::f(); g::h(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	calls, err := (&RustAdapter{}).ExtractCalls(path)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{}
	for _, c := range calls {
		byName[c.Callee.Name] = c.Callee.Package
	}
	for _, name := range []string{"b", "d", "f", "h"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("missing call %s in %#v", name, byName)
		}
	}
	root := filepath.ToSlash(dir)
	if byName["f"] != root+"/e" {
		t.Fatalf("crate::e::f package = %q, want %q", byName["f"], root+"/e")
	}
	if byName["h"] != root+"/g" {
		t.Fatalf("g::h package = %q, want %q", byName["h"], root+"/g")
	}
}

// Calls inside macro bodies (format!/println!/...) parse as token trees and
// must still be extracted.
func TestRustAdapterExtractCallsInMacros(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.rs")
	src := `fn a() {
    format!("x {}", c());
    println!("{}", self.f());
    vec![g()];
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	calls, err := (&RustAdapter{}).ExtractCalls(path)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]bool{}
	for _, c := range calls {
		byName[c.Callee.Name] = true
	}
	for _, name := range []string{"c", "f", "g"} {
		if !byName[name] {
			t.Fatalf("missing macro-body call %s in %#v", name, byName)
		}
	}
	// The macros themselves are not calls.
	for _, name := range []string{"format", "println", "vec"} {
		if byName[name] {
			t.Fatalf("macro %s must not be reported as a call", name)
		}
	}
}

func TestRustAdapterConventions(t *testing.T) {
	a := &RustAdapter{}
	if !a.IsTestFile(filepath.Join("pkg", "tests", "api.rs")) {
		t.Fatal("tests dir should be test file")
	}
	if !a.IsTestFile("foo_test.rs") {
		t.Fatal("_test.rs should be test file")
	}
	if a.Language() != model.LanguageRust {
		t.Fatal("wrong language")
	}
}

func TestRustAdapterPackage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src", "parser")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "mod.rs")
	if err := os.WriteFile(file, []byte(`pub fn helper(){}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := (&RustAdapter{}).ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.ToSlash(path)
	if fs.Package != want {
		t.Fatalf("package = %q, want %q", fs.Package, want)
	}
	for _, s := range fs.Symbols {
		if s.Package != want {
			t.Fatalf("symbol %s package = %q, want %q", s.Name, s.Package, want)
		}
	}

	// A plain file forms its own submodule named after the file stem.
	plain := filepath.Join(path, "ast.rs")
	if err := os.WriteFile(plain, []byte(`pub fn parse(){}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err = (&RustAdapter{}).ParseFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	if want := want + "/ast"; fs.Package != want {
		t.Fatalf("plain file package = %q, want %q", fs.Package, want)
	}

	// Crate roots stay at their directory.
	for _, root := range []string{"lib.rs", "main.rs"} {
		file := filepath.Join(dir, "src", root)
		if err := os.WriteFile(file, []byte(`pub fn root_fn(){}`), 0o644); err != nil {
			t.Fatal(err)
		}
		fs, err := (&RustAdapter{}).ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.ToSlash(filepath.Join(dir, "src")); fs.Package != want {
			t.Fatalf("%s package = %q, want %q", root, fs.Package, want)
		}
	}
}

func TestRustAdapterEnumTypeConstStatic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.rs")
	source := `pub enum Color { Red, Green(u8), Blue { r: u8 } }
type Alias = u64;
pub const MAX: u32 = 10;
static NAME: &str = "x";
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := (&RustAdapter{}).ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*model.Symbol{}
	var variants []*model.Symbol
	for _, s := range fs.Symbols {
		byName[s.Name] = s
		if s.Owner == "Color" {
			variants = append(variants, s)
		}
	}
	for name, kind := range map[string]model.SymbolKind{
		"Color": model.SymbolKindStruct,
		"Alias": model.SymbolKindStruct,
		"MAX":   model.SymbolKindConstant,
		"NAME":  model.SymbolKindConstant,
	} {
		sym, ok := byName[name]
		if !ok {
			t.Fatalf("missing symbol %s in %#v", name, fs.Symbols)
		}
		if sym.Kind != kind {
			t.Fatalf("%s kind = %s, want %s", name, sym.Kind, kind)
		}
	}
	if len(variants) != 3 {
		t.Fatalf("expected 3 enum variants owned by Color, got %#v", variants)
	}
	for _, v := range variants {
		if v.Kind != model.SymbolKindConstant {
			t.Fatalf("variant %s kind = %s, want constant", v.Name, v.Kind)
		}
	}
}

func TestRustAdapterTraitImplEdge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.rs")
	source := `pub trait Greet {}
struct Foo {}
impl Greet for Foo {}
impl Foo {
    fn inherent(&self) {}
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	calls, err := (&RustAdapter{}).ExtractCalls(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range calls {
		if c.EdgeKind != model.EdgeKindInheritance {
			continue
		}
		found = true
		if c.Callee == nil || c.Callee.Name != "Greet" || c.Callee.Kind != model.SymbolKindInterface {
			t.Fatalf("inheritance callee = %#v, want interface Greet", c.Callee)
		}
		if c.Caller == nil || c.Caller.Name != "Foo" {
			t.Fatalf("inheritance caller = %#v, want Foo", c.Caller)
		}
	}
	if !found {
		t.Fatalf("expected an inheritance edge for impl Greet for Foo, got %#v", calls)
	}
}

func TestRustAdapterCfgTestDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.rs")
	source := `pub fn helper() {}

#[cfg(test)]
mod tests {
    fn it_works() {}
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := (&RustAdapter{}).ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.Symbols) == 0 {
		t.Fatal("expected symbols")
	}
	for _, s := range fs.Symbols {
		if !s.IsTest {
			t.Fatalf("symbol %s should be marked as test in a #[cfg(test)] file", s.Name)
		}
	}

	plain := filepath.Join(dir, "plain.rs")
	if err := os.WriteFile(plain, []byte(`pub fn helper() {}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err = (&RustAdapter{}).ParseFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range fs.Symbols {
		if s.IsTest {
			t.Fatalf("symbol %s should not be marked as test", s.Name)
		}
	}
}
