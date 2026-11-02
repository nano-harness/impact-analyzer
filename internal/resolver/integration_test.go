package resolver_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/lang/javascript"
	"github.com/nano-harness/impact-analyzer/internal/lang/typescript"
	"github.com/nano-harness/impact-analyzer/internal/resolver"
)

// TestIntegration_JavaScriptImportResolution demonstrates end-to-end import resolution for JavaScript
func TestIntegration_JavaScriptImportResolution(t *testing.T) {
	// Create a test project structure
	tmpDir := t.TempDir()

	// Create JavaScript files with imports
	files := map[string]string{
		"src/index.js": `
import { helper } from './utils/helper';
import { Component } from './components/Component';

export function main() {
    helper();
    Component();
}
`,
		"src/utils/helper.js": `
export function helper() {
    console.log("helper");
}
`,
		"src/components/Component.js": `
export function Component() {
    console.log("Component");
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

	// Step 1: Parse the file to extract imports
	jsAdapter := &javascript.JavaScriptAdapter{}
	indexFile := filepath.Join(tmpDir, "src/index.js")

	imports, err := jsAdapter.ResolveImports(indexFile)
	if err != nil {
		t.Fatalf("Failed to resolve imports: %v", err)
	}

	// Verify we found the imports
	expectedImports := []string{"./utils/helper", "./components/Component"}
	if len(imports) != len(expectedImports) {
		t.Errorf("Expected %d imports, got %d", len(expectedImports), len(imports))
	}

	// Step 2: Use resolver to convert import strings to file paths
	factory := resolver.NewResolverFactory(tmpDir)
	resolvedImports := factory.ResolveImportsForFile(
		indexFile,
		jsAdapter.Language(),
		imports,
	)

	// Verify the imports were resolved correctly
	expected := map[string]string{
		"./utils/helper":         filepath.Join(tmpDir, "src/utils/helper.js"),
		"./components/Component": filepath.Join(tmpDir, "src/components/Component.js"),
	}

	for importPath, expectedPath := range expected {
		if resolvedPath, ok := resolvedImports[importPath]; !ok {
			t.Errorf("Import %q was not resolved", importPath)
		} else if resolvedPath != expectedPath {
			t.Errorf("Import %q resolved to %q, want %q", importPath, resolvedPath, expectedPath)
		}
	}
}

// TestIntegration_TypeScriptImportResolution demonstrates end-to-end import resolution for TypeScript
func TestIntegration_TypeScriptImportResolution(t *testing.T) {
	// Create a test project structure
	tmpDir := t.TempDir()

	// Create TypeScript files with imports
	files := map[string]string{
		"src/index.ts": `
import { Service } from './services/Service';
import { utils } from './utils';

export function main() {
    const service = new Service();
    utils.helper();
}
`,
		"src/services/Service.ts": `
export class Service {
    run() {
        console.log("service running");
    }
}
`,
		"src/utils/index.ts": `
export const utils = {
    helper: () => console.log("helper")
};
`,
		"package.json":  `{"name": "test-project"}`,
		"tsconfig.json": `{"compilerOptions": {}}`,
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

	// Step 1: Parse the file to extract imports
	tsAdapter := &typescript.TypeScriptAdapter{}
	indexFile := filepath.Join(tmpDir, "src/index.ts")

	imports, err := tsAdapter.ResolveImports(indexFile)
	if err != nil {
		t.Fatalf("Failed to resolve imports: %v", err)
	}

	// Step 2: Use resolver to convert import strings to file paths
	factory := resolver.NewResolverFactory(tmpDir)
	resolvedImports := factory.ResolveImportsForFile(
		indexFile,
		tsAdapter.Language(),
		imports,
	)

	// Verify the imports were resolved correctly
	expected := map[string]string{
		"./services/Service": filepath.Join(tmpDir, "src/services/Service.ts"),
		"./utils":            filepath.Join(tmpDir, "src/utils/index.ts"),
	}

	for importPath, expectedPath := range expected {
		if resolvedPath, ok := resolvedImports[importPath]; !ok {
			t.Errorf("Import %q was not resolved", importPath)
		} else if resolvedPath != expectedPath {
			t.Errorf("Import %q resolved to %q, want %q", importPath, resolvedPath, expectedPath)
		}
	}
}

// TestIntegration_FindProjectRoot demonstrates finding project root
func TestIntegration_FindProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a project structure
	files := []string{
		"package.json",
		"src/components/deep/nested/Component.tsx",
	}

	for _, path := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Find project root from a deeply nested file
	nestedFile := filepath.Join(tmpDir, "src/components/deep/nested/Component.tsx")
	projectRoot := resolver.FindProjectRoot(nestedFile)

	if projectRoot != tmpDir {
		t.Errorf("FindProjectRoot() = %q, want %q", projectRoot, tmpDir)
	}
}
