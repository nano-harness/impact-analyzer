package javascript

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestJavaScriptAdapter_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.js")

	testCode := `// Test file
const util = require('./util');

function helper() {
    console.log("helper");
}

export function publicFunc() {
    console.log("public");
}

export class MyClass {
    method() {
        console.log("method");
    }
}

class InternalClass {
    method() {
        console.log("internal");
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify language
	if fileSymbols.Language != model.LanguageJavaScript {
		t.Errorf("Expected LanguageJavaScript, got %s", fileSymbols.Language)
	}

	// Verify symbols were found
	if len(fileSymbols.Symbols) == 0 {
		t.Errorf("Expected to find symbols")
	}

	// Check exported function
	foundExportedFunc := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "publicFunc" && sym.Kind == model.SymbolKindFunction {
			foundExportedFunc = true
			if !sym.IsExported {
				t.Errorf("publicFunc should be marked as exported")
			}
			break
		}
	}
	if !foundExportedFunc {
		t.Errorf("publicFunc not found in symbols")
	}

	// Check non-exported function
	foundHelperFunc := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "helper" && sym.Kind == model.SymbolKindFunction {
			foundHelperFunc = true
			if sym.IsExported {
				t.Errorf("helper should not be marked as exported")
			}
			break
		}
	}
	if !foundHelperFunc {
		t.Errorf("helper not found in symbols")
	}

	// Check exported class
	foundExportedClass := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "MyClass" && sym.Kind == model.SymbolKindClass {
			foundExportedClass = true
			if !sym.IsExported {
				t.Errorf("MyClass should be marked as exported")
			}
			break
		}
	}
	if !foundExportedClass {
		t.Errorf("MyClass not found in symbols")
	}

	// Check non-exported class
	foundInternalClass := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "InternalClass" && sym.Kind == model.SymbolKindClass {
			foundInternalClass = true
			if sym.IsExported {
				t.Errorf("InternalClass should not be marked as exported")
			}
			break
		}
	}
	if !foundInternalClass {
		t.Errorf("InternalClass not found in symbols")
	}
}

func TestJavaScriptAdapter_ParseFile_NamedExports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "named-exports.js")

	testCode := `function foo() {}
function bar() {}
function baz() {}

export { foo, bar };
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Check foo is exported
	foundFoo := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "foo" {
			foundFoo = true
			if !sym.IsExported {
				t.Errorf("foo should be marked as exported (named export)")
			}
			break
		}
	}
	if !foundFoo {
		t.Errorf("foo not found in symbols")
	}

	// Check bar is exported
	foundBar := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "bar" {
			foundBar = true
			if !sym.IsExported {
				t.Errorf("bar should be marked as exported (named export)")
			}
			break
		}
	}
	if !foundBar {
		t.Errorf("bar not found in symbols")
	}

	// Check baz is not exported
	foundBaz := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "baz" {
			foundBaz = true
			if sym.IsExported {
				t.Errorf("baz should not be marked as exported")
			}
			break
		}
	}
	if !foundBaz {
		t.Errorf("baz not found in symbols")
	}
}

func TestJavaScriptAdapter_IsTestFile(t *testing.T) {
	adapter := &JavaScriptAdapter{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.js", false},
		{"app.test.js", true},
		{"component.spec.js", true},
		{"helper.js", false},
		{"__tests__/main.js", true},
		{"/path/to/__tests__/file.js", true},
		{"test.js", false}, // Not a test file by itself
	}

	for _, tt := range tests {
		result := adapter.IsTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestJavaScriptAdapter_isTestSymbolName(t *testing.T) {
	adapter := &JavaScriptAdapter{}

	tests := []struct {
		name     string
		expected bool
	}{
		{"testSomething", true},
		{"shouldDoSomething", true},
		{"itWorks", true},
		{"describe", true},
		{"publicMethod", false},
		{"helper", false},
		{"", false},
	}

	for _, tt := range tests {
		result := adapter.isTestSymbolName(tt.name)
		if result != tt.expected {
			t.Errorf("isTestSymbolName(%q) = %v, want %v", tt.name, result, tt.expected)
		}
	}
}

func TestJavaScriptAdapter_ExtractCalls(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calls.js")

	testCode := `function caller() {
    targetMethod();
    obj.someMethod();
    console.log("test");
}

function targetMethod() {
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	// Verify we extracted some calls
	if len(calls) == 0 {
		t.Errorf("Expected to find call sites")
	}
}

func TestJavaScriptAdapter_ResolveImports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "imports.js")

	testCode := `import { foo } from './util';
import React from 'react';

export function component() {
    return foo();
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	imports, err := adapter.ResolveImports(testFile)
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}

	// Verify we found imports
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d", len(imports))
	}

	// Check for specific imports
	foundUtil := false
	foundReact := false
	for _, imp := range imports {
		if imp == "./util" {
			foundUtil = true
		}
		if imp == "react" {
			foundReact = true
		}
	}

	if !foundUtil {
		t.Errorf("Expected to find import './util'")
	}
	if !foundReact {
		t.Errorf("Expected to find import 'react'")
	}
}

func TestJavaScriptAdapter_Language(t *testing.T) {
	adapter := &JavaScriptAdapter{}
	if adapter.Language() != model.LanguageJavaScript {
		t.Errorf("Expected LanguageJavaScript, got %s", adapter.Language())
	}
}

func TestJavaScriptAdapter_IsTest_NonTestSymbol_InTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "helper.test.js")
	code := `function helper() {}
class HelperClass {}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fs, err := (&JavaScriptAdapter{}).ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	for _, name := range []string{"helper", "HelperClass"} {
		sym := findJavaScriptSymbol(fs.Symbols, name)
		if sym == nil {
			t.Fatalf("%s not found in symbols: %#v", name, fs.Symbols)
		}
		if !sym.IsTest {
			t.Fatalf("%s should be marked as test in a .test.js file", name)
		}
	}
}

func findJavaScriptSymbol(symbols []*model.Symbol, name string) *model.Symbol {
	for _, sym := range symbols {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

func TestJavaScriptAdapter_IsExported(t *testing.T) {
	adapter := &JavaScriptAdapter{}

	exportedNames := map[string]bool{"publicFunc": true, "MyClass": true}

	tests := []struct {
		name     string
		context  interface{}
		expected bool
	}{
		{"publicFunc", exportedNames, true},
		{"MyClass", exportedNames, true},
		{"helper", exportedNames, false},
		{"publicFunc", nil, false},    // No export context: not exported
		{"publicFunc", "junk", false}, // Wrong context type: not exported
	}

	for _, tt := range tests {
		result := adapter.IsExported(tt.name, tt.context)
		if result != tt.expected {
			t.Errorf("IsExported(%q, %v) = %v, want %v", tt.name, tt.context, result, tt.expected)
		}
	}
}

// Test receiver owner hints on method calls: callee.Owner carries the
// best-effort receiver variable/property name (not a resolved type).
func TestJavaScriptAdapter_ExtractCalls_ReceiverOwnerHint(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calls.js")

	testCode := `function caller(repo) {
    targetMethod();
    obj.someMethod();
    this.service.find(1);
    getSvc().load(2);
}

function targetMethod() {
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	// owners maps "name:line" -> callee.Owner
	owners := make(map[string]string)
	for _, call := range calls {
		if call.Callee == nil {
			continue
		}
		owners[call.Callee.Name+":"+strconv.Itoa(call.Line)] = call.Callee.Owner
	}

	want := map[string]string{
		"targetMethod:2": "",        // bare function call -> no receiver
		"someMethod:3":   "obj",     // obj.someMethod()
		"find:4":         "service", // this.service.find(1) -> last chain segment
		"load:5":         "",        // getSvc().load(2) -> complex receiver, no hint
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

func TestJavaScriptAdapter_ParseFile_ArrowFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "arrows.js")

	testCode := `export const handler = async () => {
    return 1;
};

const App = () => {
    return null;
};

var legacy = function() {
    return 2;
};

const count = 42;
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	handler := findJavaScriptSymbol(fileSymbols.Symbols, "handler")
	if handler == nil {
		t.Fatalf("handler (exported arrow function) not found in symbols: %#v", fileSymbols.Symbols)
	}
	if handler.Kind != model.SymbolKindFunction {
		t.Errorf("handler kind = %s, want function", handler.Kind)
	}
	if !handler.IsExported {
		t.Errorf("handler should be marked as exported")
	}

	app := findJavaScriptSymbol(fileSymbols.Symbols, "App")
	if app == nil {
		t.Fatalf("App (arrow function) not found in symbols")
	}
	if app.Kind != model.SymbolKindFunction {
		t.Errorf("App kind = %s, want function", app.Kind)
	}
	if app.IsExported {
		t.Errorf("App should not be marked as exported")
	}

	legacy := findJavaScriptSymbol(fileSymbols.Symbols, "legacy")
	if legacy == nil {
		t.Fatalf("legacy (var function expression) not found in symbols")
	}
	if legacy.Kind != model.SymbolKindFunction {
		t.Errorf("legacy kind = %s, want function", legacy.Kind)
	}

	if findJavaScriptSymbol(fileSymbols.Symbols, "count") != nil {
		t.Errorf("plain data variable 'count' should not be emitted as a symbol")
	}
}

func TestJavaScriptAdapter_ParseFile_ReExports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "reexports.js")

	testCode := `export * from './x';
export { a } from './y';
import { b } from './z';
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	imports, err := adapter.ResolveImports(testFile)
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}

	found := make(map[string]bool)
	for _, imp := range imports {
		found[imp] = true
	}
	for _, want := range []string{"./x", "./y", "./z"} {
		if !found[want] {
			t.Errorf("Expected import %q, got %v", want, imports)
		}
	}
}

func TestJavaScriptAdapter_ParseFile_ExportDefault(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "default.js")

	testCode := `function foo() {}
export default foo;

export default function bar() {}

function baz() {}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	foo := findJavaScriptSymbol(fileSymbols.Symbols, "foo")
	if foo == nil {
		t.Fatalf("foo not found in symbols")
	}
	if !foo.IsExported {
		t.Errorf("foo should be marked as exported (export default foo)")
	}

	bar := findJavaScriptSymbol(fileSymbols.Symbols, "bar")
	if bar == nil {
		t.Fatalf("bar not found in symbols")
	}
	if !bar.IsExported {
		t.Errorf("bar should be marked as exported (export default function bar)")
	}

	baz := findJavaScriptSymbol(fileSymbols.Symbols, "baz")
	if baz == nil {
		t.Fatalf("baz not found in symbols")
	}
	if baz.IsExported {
		t.Errorf("baz should not be marked as exported")
	}
}

func TestJavaScriptAdapter_ExtractCalls_JSXComponents(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "app.jsx")

	testCode := `const App = () => {
    return (
        <div>
            <UserCard name="x" />
            <Foo.Bar />
            <span>text</span>
        </div>
    );
};
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaScriptAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	var userCard, fooBar *model.CallSite
	for _, call := range calls {
		if call.Callee == nil {
			continue
		}
		switch call.Callee.Name {
		case "UserCard":
			userCard = call
		case "Bar":
			fooBar = call
		case "div", "span":
			t.Errorf("intrinsic element %q should not produce a call edge", call.Callee.Name)
		}
	}

	if userCard == nil {
		t.Fatalf("no JSX edge for <UserCard/> (all calls: %#v)", calls)
	}
	if userCard.EdgeKind != model.EdgeKindTypeRef {
		t.Errorf("UserCard edge kind = %q, want type_ref", userCard.EdgeKind)
	}

	if fooBar == nil {
		t.Fatalf("no JSX edge for <Foo.Bar/>")
	}
	if fooBar.Callee.Owner != "Foo" {
		t.Errorf("<Foo.Bar/> owner = %q, want Foo", fooBar.Callee.Owner)
	}
}

func TestJavaScriptAdapter_IsTestFile_Variants(t *testing.T) {
	adapter := &JavaScriptAdapter{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"app.test.jsx", true},
		{"app.spec.jsx", true},
		{"app.test.mjs", true},
		{"app.spec.mjs", true},
		{"app.test.cjs", true},
		{"app.spec.cjs", true},
		{"app.jsx", false},
		{"app.mjs", false},
		{"app.cjs", false},
	}

	for _, tt := range tests {
		result := adapter.IsTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}
