package java

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestJavaAdapter_ParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Test.java")

	testCode := `package com.example;

import java.util.List;
import java.util.ArrayList;

public class TestClass {
    public void publicMethod() {
        System.out.println("public");
    }

    private void privateMethod() {
        System.out.println("private");
    }

    public void testSomething() {
        // test method
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify package
	if fileSymbols.Package != "com.example" {
		t.Errorf("Expected package 'com.example', got '%s'", fileSymbols.Package)
	}

	// Verify language
	if fileSymbols.Language != model.LanguageJava {
		t.Errorf("Expected LanguageJava, got %s", fileSymbols.Language)
	}

	// Verify symbols were found
	if len(fileSymbols.Symbols) == 0 {
		t.Errorf("Expected to find symbols")
	}

	// Check if class was found
	foundClass := false
	for _, sym := range fileSymbols.Symbols {
		if sym.Name == "TestClass" && sym.Kind == model.SymbolKindClass {
			foundClass = true
			break
		}
	}
	if !foundClass {
		t.Errorf("TestClass not found in symbols")
	}
}

func TestJavaAdapter_IsTestFile(t *testing.T) {
	adapter := &JavaAdapter{}

	tests := []struct {
		path     string
		expected bool
	}{
		{"Main.java", false},
		{"MainTest.java", true},
		{"TestMain.java", false}, // Java convention is suffix only
		{"SomeTests.java", true},
		{"Helper.java", false},
		{"/path/to/MyTest.java", true},
	}

	for _, tt := range tests {
		result := adapter.IsTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestJavaAdapter_IsExported(t *testing.T) {
	adapter := &JavaAdapter{}

	tests := []struct {
		name     string
		context  interface{}
		expected bool
	}{
		{"publicMethod", "public void publicMethod()", true},
		{"privateMethod", "private void privateMethod()", false},
		{"method", "", false}, // Default is false for Java (needs public keyword)
		{"protectedMethod", "protected void protectedMethod()", false},
	}

	for _, tt := range tests {
		result := adapter.IsExported(tt.name, tt.context)
		if result != tt.expected {
			t.Errorf("IsExported(%q, %q) = %v, want %v", tt.name, tt.context, result, tt.expected)
		}
	}
}

func TestJavaAdapter_ExtractCalls(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Caller.java")

	testCode := `package com.example;

public class Caller {
    public void caller() {
        targetMethod();
        obj.someMethod();
        System.out.println("test");
    }

    public void targetMethod() {
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	// Just verify it doesn't error - call extraction behavior may vary
	_ = calls
}

func TestJavaAdapter_ResolveImports(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Imports.java")

	testCode := `package com.example;

import java.util.List;
import java.util.ArrayList;
import java.io.IOException;

public class Imports {
    public void method() {
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	imports, err := adapter.ResolveImports(testFile)
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}

	// Verify method works without errors
	_ = imports
}

func TestJavaAdapter_Language(t *testing.T) {
	adapter := &JavaAdapter{}
	if adapter.Language() != model.LanguageJava {
		t.Errorf("Expected LanguageJava, got %s", adapter.Language())
	}
}

func TestJavaAdapter_IsTest_NonTestSymbol_InTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "HelperTest.java")
	code := `package com.example;

class HelperTest {
    void helperMethod() {}
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	fs, err := (&JavaAdapter{}).ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	for _, name := range []string{"HelperTest", "helperMethod"} {
		sym := findJavaSymbol(fs.Symbols, name)
		if sym == nil {
			t.Fatalf("%s not found in symbols: %#v", name, fs.Symbols)
		}
		if !sym.IsTest {
			t.Fatalf("%s should be marked as test in a *Test.java file", name)
		}
	}
}

func findJavaSymbol(symbols []*model.Symbol, name string) *model.Symbol {
	for _, sym := range symbols {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

// Test field type-ref edges (Commit D - #9)
func TestJavaAdapter_FieldTypeRefs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Controller.java")

	testCode := `package com.example;

import org.springframework.beans.factory.annotation.Autowired;

class UserService {}

class Controller {
    @Autowired
    private UserService userService;

    private String name;
    private int count;
    private List<String> items;
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	// Should have a type-ref edge to UserService
	var foundUserService bool
	for _, call := range calls {
		if call.EdgeKind == model.EdgeKindTypeRef &&
			call.Callee != nil &&
			call.Callee.Name == "UserService" {
			foundUserService = true
			if call.Callee.Kind != model.SymbolKindClass {
				t.Errorf("Expected UserService to be SymbolKindClass, got %s", call.Callee.Kind)
			}
			break
		}
	}

	if !foundUserService {
		t.Errorf("Expected type-ref edge to UserService, found calls: %#v", calls)
	}

	// Should NOT have edges for built-in types (String, int)
	for _, call := range calls {
		if call.EdgeKind == model.EdgeKindTypeRef && call.Callee != nil {
			name := call.Callee.Name
			if name == "String" || name == "int" || name == "List" {
				t.Errorf("Should not create type-ref edge for built-in type: %s", name)
			}
		}
	}
}

func TestJavaAdapter_ParseFile_EnumAndRecord(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Shapes.java")

	testCode := `package com.example;

public enum Color {
    RED, GREEN, BLUE;

    public String label() {
        return name();
    }
}

public record Point(int x, int y) {
    public int sum() {
        return x + y;
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	for _, name := range []string{"Color", "Point"} {
		found := false
		for _, sym := range fileSymbols.Symbols {
			if sym.Name == name && sym.Kind == model.SymbolKindClass {
				found = true
				if !sym.IsExported {
					t.Errorf("%s should be marked as exported (public)", name)
				}
				if sym.Package != "com.example" {
					t.Errorf("%s package = %q, want com.example", name, sym.Package)
				}
				break
			}
		}
		if !found {
			t.Errorf("%s not found in symbols", name)
		}
	}
}

// Test receiver owner hints on method calls: callee.Owner carries the
// best-effort receiver variable/field name (not a resolved type).
func TestJavaAdapter_ExtractCalls_ReceiverOwnerHint(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Caller.java")

	testCode := `package com.example;

public class Caller {
    private UserService userService;

    public void run(UserRepository repo) {
        userService.find(1);
        this.userService.find(2);
        repo.save(3);
        find(4);
        getService().find(5);
    }

    private UserService getService() { return userService; }
    private void find(int id) {}
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	// owners maps "name:line" -> callee.Owner for plain call edges
	owners := make(map[string]string)
	for _, call := range calls {
		if call.Callee == nil || call.EdgeKind != "" {
			continue
		}
		owners[call.Callee.Name+":"+strconv.Itoa(call.Line)] = call.Callee.Owner
	}

	want := map[string]string{
		"find:7":  "userService", // userService.find(1)
		"find:8":  "userService", // this.userService.find(2) -> last chain segment
		"save:9":  "repo",        // repo.save(3)
		"find:10": "Caller",      // bare find(4) -> implicit this -> enclosing class
		"find:11": "",            // getService().find(5) -> complex receiver, no hint
	}
	for key, wantOwner := range want {
		got, ok := owners[key]
		if !ok {
			t.Errorf("no method call extracted for %s (all calls: %#v)", key, calls)
			continue
		}
		if got != wantOwner {
			t.Errorf("callee %s Owner = %q, want %q", key, got, wantOwner)
		}
	}
}

// Test method owner assignment (#13): same-named methods in different
// classes must carry their own enclosing class as Owner, which makes their
// QualifiedName distinct within a shared package.
func TestJavaAdapter_ParseFile_MethodOwners(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Shapes.java")

	testCode := `package com.example;

public class Helper {
    public String greet(String name) {
        return "hello " + name;
    }
}

class Other {
    public String greet(String name) {
        return "other " + name;
    }

    public String useOwn(String name) {
        return greet(name);
    }
}

class Outer {
    void outerMethod() {
    }

    static class Inner {
        void greet() {
        }
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	fileSymbols, err := adapter.ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	owners := make(map[string]string) // QualifiedName -> Owner (sanity)
	var greetOwners []string
	for _, sym := range fileSymbols.Symbols {
		if sym.Kind != model.SymbolKindMethod {
			continue
		}
		owners[sym.QualifiedName()] = sym.Owner
		switch sym.Name {
		case "greet":
			greetOwners = append(greetOwners, sym.Owner)
		case "useOwn":
			if sym.Owner != "Other" {
				t.Errorf("useOwn Owner = %q, want %q", sym.Owner, "Other")
			}
		case "outerMethod":
			if sym.Owner != "Outer" {
				t.Errorf("outerMethod Owner = %q, want %q", sym.Owner, "Outer")
			}
		}
	}

	wantGreetOwners := map[string]bool{"Helper": false, "Other": false, "Inner": false}
	for _, owner := range greetOwners {
		if _, ok := wantGreetOwners[owner]; !ok {
			t.Errorf("unexpected greet owner %q (all: %v)", owner, greetOwners)
			continue
		}
		wantGreetOwners[owner] = true
	}
	for owner, seen := range wantGreetOwners {
		if !seen {
			t.Errorf("no greet method with Owner %q (all: %v)", owner, greetOwners)
		}
	}

	// Same-named methods must not share a qualified identity.
	for _, qname := range []string{
		"com.example.Helper.greet",
		"com.example.Other.greet",
		"com.example.Inner.greet", // nested: Owner is the nearest enclosing class
	} {
		if _, ok := owners[qname]; !ok {
			t.Errorf("expected qualified name %q, got %v", qname, owners)
		}
	}
}

// Test implicit-this owner hints: unqualified calls and explicit this calls
// hint the innermost enclosing class; nested classes hint the nested class.
func TestJavaAdapter_ExtractCalls_ImplicitThisHint(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Caller.java")

	testCode := `package com.example;

public class Caller {
    public void run() {
        greet("a");
        this.greet("b");
        super.toString();
    }

    private void greet(String name) {
    }

    static class Inner {
        void call() {
            greet("c");
        }

        private void greet(String name) {
        }
    }
}
`

	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &JavaAdapter{}
	calls, err := adapter.ExtractCalls(testFile)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	owners := make(map[string]string)
	for _, call := range calls {
		if call.Callee == nil || call.EdgeKind != "" {
			continue
		}
		owners[call.Callee.Name+":"+strconv.Itoa(call.Line)] = call.Callee.Owner
	}

	want := map[string]string{
		"greet:5":    "Caller", // bare greet("a") -> implicit this
		"greet:6":    "Caller", // this.greet("b") -> explicit this
		"toString:7": "",       // super receiver -> no hint
		"greet:15":   "Inner",  // bare call inside nested class -> nested class
	}
	for key, wantOwner := range want {
		got, ok := owners[key]
		if !ok {
			t.Errorf("no method call extracted for %s (all calls: %#v)", key, calls)
			continue
		}
		if got != wantOwner {
			t.Errorf("callee %s Owner = %q, want %q", key, got, wantOwner)
		}
	}
}
