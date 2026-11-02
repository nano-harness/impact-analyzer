package resolver_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/lang/java"
	"github.com/nano-harness/impact-analyzer/internal/lang/javascript"
	"github.com/nano-harness/impact-analyzer/internal/lang/typescript"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/resolver"
)

// TestEndToEnd_AllFeatures demonstrates all features working together:
// - Phase 1: Export detection for JS/TS
// - Phase 2: Generic type parameter support
// - Phase 3: Import resolution
func TestEndToEnd_AllFeatures(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a comprehensive TypeScript project
	files := map[string]string{
		"src/index.ts": `
import { Service } from './services/Service';
import { Helper } from './utils/Helper';

export function main<T>(data: T): void {
    const service = new Service();
    Helper.process(data);
}
`,
		"src/services/Service.ts": `
export class Service<T = string> {
    private data: T;

    constructor() {}

    public process<U>(input: U): void {
        console.log(input);
    }
}

class InternalService {
    run() {}
}
`,
		"src/utils/Helper.ts": `
export class Helper {
    static process<T>(data: T): T {
        return data;
    }
}

function internalHelper() {
    return "internal";
}
`,
		"package.json": `{"name": "test-project"}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	tsAdapter := &typescript.TypeScriptAdapter{}
	factory := resolver.NewResolverFactory(tmpDir)

	// Test 1: Parse Service.ts and verify export detection + generic types
	t.Run("Export Detection and Generic Types", func(t *testing.T) {
		serviceFile := filepath.Join(tmpDir, "src/services/Service.ts")
		fileSymbols, err := tsAdapter.ParseFile(serviceFile)
		if err != nil {
			t.Fatalf("Failed to parse file: %v", err)
		}

		// Find the Service class
		var serviceClass *model.Symbol
		var internalServiceClass *model.Symbol
		for _, sym := range fileSymbols.Symbols {
			if sym.Name == "Service" {
				serviceClass = sym
			}
			if sym.Name == "InternalService" {
				internalServiceClass = sym
			}
		}

		// Verify Service class is exported
		if serviceClass == nil {
			t.Fatal("Service class not found")
		}
		if !serviceClass.IsExported {
			t.Error("Service class should be exported")
		}

		// Verify Service has generic type parameter
		if len(serviceClass.TypeParameters) != 1 || serviceClass.TypeParameters[0] != "T" {
			t.Errorf("Service class should have type parameter T, got %v", serviceClass.TypeParameters)
		}

		// Verify InternalService is not exported
		if internalServiceClass == nil {
			t.Fatal("InternalService class not found")
		}
		if internalServiceClass.IsExported {
			t.Error("InternalService should not be exported")
		}
	})

	// Test 2: Parse index.ts and verify import resolution
	t.Run("Import Resolution", func(t *testing.T) {
		indexFile := filepath.Join(tmpDir, "src/index.ts")

		// Get imports
		imports, err := tsAdapter.ResolveImports(indexFile)
		if err != nil {
			t.Fatalf("Failed to resolve imports: %v", err)
		}

		// Resolve imports to file paths
		resolvedImports := factory.ResolveImportsForFile(
			indexFile,
			model.LanguageTypeScript,
			imports,
		)

		// Verify imports were resolved
		expectedResolutions := map[string]string{
			"./services/Service": filepath.Join(tmpDir, "src/services/Service.ts"),
			"./utils/Helper":     filepath.Join(tmpDir, "src/utils/Helper.ts"),
		}

		for imp, expectedPath := range expectedResolutions {
			if resolvedPath, ok := resolvedImports[imp]; !ok {
				t.Errorf("Import %q was not resolved", imp)
			} else if resolvedPath != expectedPath {
				t.Errorf("Import %q resolved to %q, want %q", imp, resolvedPath, expectedPath)
			}
		}
	})

	// Test 3: Parse index.ts and verify exported functions with generics
	t.Run("Exported Functions with Generics", func(t *testing.T) {
		indexFile := filepath.Join(tmpDir, "src/index.ts")
		fileSymbols, err := tsAdapter.ParseFile(indexFile)
		if err != nil {
			t.Fatalf("Failed to parse file: %v", err)
		}

		// Find the main function
		var mainFunc *model.Symbol
		for _, sym := range fileSymbols.Symbols {
			if sym.Name == "main" {
				mainFunc = sym
			}
		}

		if mainFunc == nil {
			t.Fatal("main function not found")
		}

		// Verify main is exported
		if !mainFunc.IsExported {
			t.Error("main function should be exported")
		}

		// Verify main has generic type parameter T
		if len(mainFunc.TypeParameters) != 1 || mainFunc.TypeParameters[0] != "T" {
			t.Errorf("main function should have type parameter T, got %v", mainFunc.TypeParameters)
		}
	})
}

// TestEndToEnd_JavaImportResolution tests Java import resolution
func TestEndToEnd_JavaImportResolution(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Java project with Maven structure
	files := map[string]string{
		"src/main/java/com/example/Main.java": `
package com.example;

import com.example.service.UserService;
import com.example.util.Helper;

public class Main {
    public static void main(String[] args) {
        UserService service = new UserService();
        Helper.process();
    }
}
`,
		"src/main/java/com/example/service/UserService.java": `
package com.example.service;

public class UserService<T> {
    public void process(T data) {
        System.out.println(data);
    }
}
`,
		"src/main/java/com/example/util/Helper.java": `
package com.example.util;

public class Helper {
    public static void process() {
        System.out.println("processing");
    }
}
`,
		"pom.xml": `<project></project>`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	javaAdapter := &java.JavaAdapter{}
	factory := resolver.NewResolverFactory(tmpDir)

	// Test: Parse Main.java and resolve imports
	t.Run("Java Import Resolution", func(t *testing.T) {
		mainFile := filepath.Join(tmpDir, "src/main/java/com/example/Main.java")

		imports, err := javaAdapter.ResolveImports(mainFile)
		if err != nil {
			t.Fatalf("Failed to get imports: %v", err)
		}

		// Resolve imports
		resolvedImports := factory.ResolveImportsForFile(
			mainFile,
			model.LanguageJava,
			imports,
		)

		// Verify imports were resolved
		expectedResolutions := map[string]string{
			"com.example.service.UserService": filepath.Join(tmpDir, "src/main/java/com/example/service/UserService.java"),
			"com.example.util.Helper":         filepath.Join(tmpDir, "src/main/java/com/example/util/Helper.java"),
		}

		for imp, expectedPath := range expectedResolutions {
			if resolvedPath, ok := resolvedImports[imp]; !ok {
				t.Errorf("Import %q was not resolved", imp)
			} else if resolvedPath != expectedPath {
				t.Errorf("Import %q resolved to %q, want %q", imp, resolvedPath, expectedPath)
			}
		}
	})

	// Test: Verify Java generic types
	t.Run("Java Generic Types", func(t *testing.T) {
		serviceFile := filepath.Join(tmpDir, "src/main/java/com/example/service/UserService.java")
		fileSymbols, err := javaAdapter.ParseFile(serviceFile)
		if err != nil {
			t.Fatalf("Failed to parse file: %v", err)
		}

		// Find the UserService class
		var userServiceClass *model.Symbol
		for _, sym := range fileSymbols.Symbols {
			if sym.Name == "UserService" && sym.Kind == model.SymbolKindClass {
				userServiceClass = sym
			}
		}

		if userServiceClass == nil {
			t.Fatal("UserService class not found")
		}

		// Verify UserService has generic type parameter T
		if len(userServiceClass.TypeParameters) != 1 || userServiceClass.TypeParameters[0] != "T" {
			t.Errorf("UserService class should have type parameter T, got %v", userServiceClass.TypeParameters)
		}
	})
}

// TestEndToEnd_JavaScriptFeatures tests JavaScript-specific features
func TestEndToEnd_JavaScriptFeatures(t *testing.T) {
	tmpDir := t.TempDir()

	// Create JavaScript files
	files := map[string]string{
		"src/index.js": `
import { service } from './service';
import { utils } from './utils';

export function main() {
    service.run();
    utils.helper();
}

function internal() {
    return "private";
}
`,
		"src/service.js": `
export const service = {
    run: () => console.log("running")
};
`,
		"src/utils/index.js": `
export const utils = {
    helper: () => console.log("helper")
};
`,
		"package.json": `{"name": "js-project"}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	jsAdapter := &javascript.JavaScriptAdapter{}
	factory := resolver.NewResolverFactory(tmpDir)

	t.Run("JavaScript Export Detection and Import Resolution", func(t *testing.T) {
		indexFile := filepath.Join(tmpDir, "src/index.js")

		// Parse file
		fileSymbols, err := jsAdapter.ParseFile(indexFile)
		if err != nil {
			t.Fatalf("Failed to parse file: %v", err)
		}

		// Verify export detection
		var mainFunc, internalFunc *model.Symbol
		for _, sym := range fileSymbols.Symbols {
			if sym.Name == "main" {
				mainFunc = sym
			}
			if sym.Name == "internal" {
				internalFunc = sym
			}
		}

		if mainFunc == nil || internalFunc == nil {
			t.Fatal("Functions not found")
		}

		if !mainFunc.IsExported {
			t.Error("main should be exported")
		}

		if internalFunc.IsExported {
			t.Error("internal should not be exported")
		}

		// Verify import resolution
		imports, err := jsAdapter.ResolveImports(indexFile)
		if err != nil {
			t.Fatalf("Failed to get imports: %v", err)
		}

		resolvedImports := factory.ResolveImportsForFile(
			indexFile,
			model.LanguageJavaScript,
			imports,
		)

		expectedResolutions := map[string]string{
			"./service": filepath.Join(tmpDir, "src/service.js"),
			"./utils":   filepath.Join(tmpDir, "src/utils/index.js"),
		}

		for imp, expectedPath := range expectedResolutions {
			if resolvedPath, ok := resolvedImports[imp]; !ok {
				t.Errorf("Import %q was not resolved", imp)
			} else if resolvedPath != expectedPath {
				t.Errorf("Import %q resolved to %q, want %q", imp, resolvedPath, expectedPath)
			}
		}
	})
}
