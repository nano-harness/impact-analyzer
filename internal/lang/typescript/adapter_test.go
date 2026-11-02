package typescript

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestTypeScriptAdapter_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ts")

	testCode := `// Test file
import { util } from './util';

function helper() {
    console.log("helper");
}

export function publicFunc(): void {
    console.log("public");
}

export class MyClass {
    method(): void {
        console.log("method");
    }
}

class InternalClass {
    method(): void {
        console.log("internal");
    }
}

export interface PublicInterface {
    name: string;
}

interface InternalInterface {
    value: number;
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify language
	if fileSymbols.Language != model.LanguageTypeScript {
		t.Errorf("Expected LanguageTypeScript, got %s", fileSymbols.Language)
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

	// Check exported interface
	foundExportedInterface := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "PublicInterface" && sym.Kind == model.SymbolKindInterface {
			foundExportedInterface = true
			if !sym.IsExported {
				t.Errorf("PublicInterface should be marked as exported")
			}
			break
		}
	}
	if !foundExportedInterface {
		t.Errorf("PublicInterface not found in symbols")
	}

	// Check non-exported interface
	foundInternalInterface := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "InternalInterface" && sym.Kind == model.SymbolKindInterface {
			foundInternalInterface = true
			if sym.IsExported {
				t.Errorf("InternalInterface should not be marked as exported")
			}
			break
		}
	}
	if !foundInternalInterface {
		t.Errorf("InternalInterface not found in symbols")
	}
}

func TestTypeScriptAdapter_ParseFile_NamedExports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "named-exports.ts")

	testCode := `function foo(): void {}
function bar(): void {}
function baz(): void {}

export { foo, bar };
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
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

func TestTypeScriptAdapter_ParseFile_TypeAliasExports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "types.ts")

	testCode := `export type PublicType = string | number;
type InternalType = boolean;
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	publicType := findTypeScriptSymbol(fileSymbols.Symbols, "PublicType")
	if publicType == nil {
		t.Fatalf("PublicType not found in symbols: %#v", fileSymbols.Symbols)
	}
	if publicType.Kind != model.SymbolKindInterface {
		t.Errorf("PublicType kind = %s, want interface (type alias)", publicType.Kind)
	}
	if !publicType.IsExported {
		t.Errorf("PublicType should be marked as exported")
	}

	internalType := findTypeScriptSymbol(fileSymbols.Symbols, "InternalType")
	if internalType == nil {
		t.Fatalf("InternalType not found in symbols")
	}
	if internalType.IsExported {
		t.Errorf("InternalType should not be marked as exported")
	}
}

func TestTypeScriptAdapter_IsTestFile(t *testing.T) {
	adapter := &TypeScriptAdapter{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.ts", false},
		{"app.test.ts", true},
		{"component.spec.ts", true},
		{"component.test.tsx", true},
		{"component.spec.tsx", true},
		{"helper.ts", false},
		{"__tests__/main.ts", true},
		{"/path/to/__tests__/file.ts", true},
		{"test.ts", false}, // Not a test file by itself
	}

	for _, tt := range tests {
		result := adapter.IsTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestTypeScriptAdapter_isTestSymbolName(t *testing.T) {
	adapter := &TypeScriptAdapter{}

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

func TestTypeScriptAdapter_ExtractCalls(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calls.ts")

	testCode := `function caller(): void {
    targetMethod();
    obj.someMethod();
    console.log("test");
}

function targetMethod(): void {
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	// Verify we extracted some calls
	if len(calls) == 0 {
		t.Errorf("Expected to find call sites")
	}
}

func TestTypeScriptAdapter_ResolveImports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "imports.ts")

	testCode := `import { foo } from './util';
import React from 'react';
import type { ComponentProps } from 'react';

export function component(): JSX.Element {
    return foo();
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	imports, err := adapter.ResolveImports(testFile)
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}

	// Verify we found imports (at least 2, may vary based on parser)
	if len(imports) < 2 {
		t.Errorf("Expected at least 2 imports, got %d", len(imports))
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

func TestTypeScriptAdapter_Language(t *testing.T) {
	adapter := &TypeScriptAdapter{}
	if adapter.Language() != model.LanguageTypeScript {
		t.Errorf("Expected LanguageTypeScript, got %s", adapter.Language())
	}
}

func TestTypeScriptAdapter_IsTest_NonTestSymbol_InTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "helper.test.ts")
	code := `function helper(): void {}
class HelperClass {}
interface HelperInterface { value: string }
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fs, err := (&TypeScriptAdapter{}).ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	for _, name := range []string{"helper", "HelperClass", "HelperInterface"} {
		sym := findTypeScriptSymbol(fs.Symbols, name)
		if sym == nil {
			t.Fatalf("%s not found in symbols: %#v", name, fs.Symbols)
		}
		if !sym.IsTest {
			t.Fatalf("%s should be marked as test in a .test.ts file", name)
		}
	}
}

func findTypeScriptSymbol(symbols []*model.Symbol, name string) *model.Symbol {
	for _, sym := range symbols {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

// Test receiver owner hints on method calls: callee.Owner carries the
// best-effort receiver variable/property name (not a resolved type).
func TestTypeScriptAdapter_ExtractCalls_ReceiverOwnerHint(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calls.ts")

	testCode := `function caller(repo: UserRepository): void {
    targetMethod();
    obj.someMethod();
    this.service.find(1);
    getSvc().load(2);
}

function targetMethod(): void {
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
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

func TestTypeScriptAdapter_ParseFile_ArrowFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "arrows.ts")

	testCode := `export const handler = async (): Promise<number> => {
    return 1;
};

const compute = (x: number): number => x * 2;

var legacy = function(): number {
    return 2;
};

const count: number = 42;
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	handler := findTypeScriptSymbol(fileSymbols.Symbols, "handler")
	if handler == nil {
		t.Fatalf("handler (exported arrow function) not found in symbols: %#v", fileSymbols.Symbols)
	}
	if handler.Kind != model.SymbolKindFunction {
		t.Errorf("handler kind = %s, want function", handler.Kind)
	}
	if !handler.IsExported {
		t.Errorf("handler should be marked as exported")
	}

	compute := findTypeScriptSymbol(fileSymbols.Symbols, "compute")
	if compute == nil {
		t.Fatalf("compute (arrow function) not found in symbols")
	}
	if compute.IsExported {
		t.Errorf("compute should not be marked as exported")
	}

	legacy := findTypeScriptSymbol(fileSymbols.Symbols, "legacy")
	if legacy == nil {
		t.Fatalf("legacy (var function expression) not found in symbols")
	}
	if legacy.Kind != model.SymbolKindFunction {
		t.Errorf("legacy kind = %s, want function", legacy.Kind)
	}

	if findTypeScriptSymbol(fileSymbols.Symbols, "count") != nil {
		t.Errorf("plain data variable 'count' should not be emitted as a symbol")
	}
}

func TestTypeScriptAdapter_ParseFile_MethodOwners(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "methods.ts")

	testCode := `class Service {
    find(id: number): string {
        return "x";
    }
}

interface Repository {
    load(id: number): void;
}

abstract class Base {
    abstract render(): void;
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	find := findTypeScriptSymbol(fileSymbols.Symbols, "find")
	if find == nil {
		t.Fatalf("find method not found in symbols: %#v", fileSymbols.Symbols)
	}
	if find.Kind != model.SymbolKindMethod {
		t.Errorf("find kind = %s, want method", find.Kind)
	}
	if find.Owner != "Service" {
		t.Errorf("find owner = %q, want Service", find.Owner)
	}

	load := findTypeScriptSymbol(fileSymbols.Symbols, "load")
	if load == nil {
		t.Fatalf("load method signature not found in symbols")
	}
	if load.Kind != model.SymbolKindMethod {
		t.Errorf("load kind = %s, want method", load.Kind)
	}
	if load.Owner != "Repository" {
		t.Errorf("load owner = %q, want Repository", load.Owner)
	}

	render := findTypeScriptSymbol(fileSymbols.Symbols, "render")
	if render == nil {
		t.Fatalf("render abstract method signature not found in symbols")
	}
	if render.Owner != "Base" {
		t.Errorf("render owner = %q, want Base", render.Owner)
	}
}

func TestTypeScriptAdapter_ParseFile_AbstractEnumNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "decls.ts")

	testCode := `export abstract class Widget {
    abstract draw(): void;
}

export enum Color {
    Red,
    Green,
}

export namespace Utils {
    export function helper(): void {}
}

enum InternalEnum {
    A,
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	widget := findTypeScriptSymbol(fileSymbols.Symbols, "Widget")
	if widget == nil {
		t.Fatalf("Widget (abstract class) not found in symbols: %#v", fileSymbols.Symbols)
	}
	if widget.Kind != model.SymbolKindClass {
		t.Errorf("Widget kind = %s, want class", widget.Kind)
	}
	if !widget.IsExported {
		t.Errorf("Widget should be marked as exported")
	}

	color := findTypeScriptSymbol(fileSymbols.Symbols, "Color")
	if color == nil {
		t.Fatalf("Color (enum) not found in symbols")
	}
	if color.Kind != model.SymbolKindClass {
		t.Errorf("Color kind = %s, want class (enums treated like classes)", color.Kind)
	}
	if !color.IsExported {
		t.Errorf("Color should be marked as exported")
	}

	utils := findTypeScriptSymbol(fileSymbols.Symbols, "Utils")
	if utils == nil {
		t.Fatalf("Utils (namespace) not found in symbols")
	}
	if utils.Kind != model.SymbolKindModule {
		t.Errorf("Utils kind = %s, want module", utils.Kind)
	}
	if !utils.IsExported {
		t.Errorf("Utils should be marked as exported")
	}

	internal := findTypeScriptSymbol(fileSymbols.Symbols, "InternalEnum")
	if internal == nil {
		t.Fatalf("InternalEnum not found in symbols")
	}
	if internal.IsExported {
		t.Errorf("InternalEnum should not be marked as exported")
	}
}

func TestTypeScriptAdapter_ParseFile_ReExports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "reexports.ts")

	testCode := `export * from './x';
export { a } from './y';
import { b } from './z';
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
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

func TestTypeScriptAdapter_ExtractCalls_JSXComponents(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "app.tsx")

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

	adapter := &TypeScriptAdapter{}
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

func TestTypeScriptAdapter_ExtractCalls_PlainTSUnaffectedByJSX(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "calls.ts")

	testCode := `function caller(): void {
    targetMethod();
}

function targetMethod(): void {
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &TypeScriptAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}
	if len(calls) == 0 {
		t.Errorf("Expected to find call sites")
	}
}

func TestTypeScriptAdapter_IsTestFile_Variants(t *testing.T) {
	adapter := &TypeScriptAdapter{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"app.test.mts", true},
		{"app.spec.mts", true},
		{"app.test.cts", true},
		{"app.spec.cts", true},
		{"app.mts", false},
		{"app.cts", false},
	}

	for _, tt := range tests {
		result := adapter.IsTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}
