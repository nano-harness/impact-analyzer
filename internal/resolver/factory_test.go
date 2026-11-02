package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestResolverFactory_GetResolver(t *testing.T) {
	factory := NewResolverFactory("/test/project")

	tests := []struct {
		name     string
		lang     model.Language
		wantType string
	}{
		{
			name:     "JavaScript resolver",
			lang:     model.LanguageJavaScript,
			wantType: "*resolver.JSResolver",
		},
		{
			name:     "TypeScript resolver",
			lang:     model.LanguageTypeScript,
			wantType: "*resolver.JSResolver",
		},
		{
			name:     "Java resolver",
			lang:     model.LanguageJava,
			wantType: "*resolver.JavaResolver",
		},
		{
			name:     "Go resolver",
			lang:     model.LanguageGo,
			wantType: "*resolver.GoResolver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := factory.GetResolver(tt.lang)
			if tt.wantType == "nil" {
				if resolver != nil {
					t.Errorf("GetResolver() = %v, want nil", resolver)
				}
			} else {
				if resolver == nil {
					t.Errorf("GetResolver() = nil, want non-nil resolver")
				}
			}
		})
	}
}

func TestResolverFactory_ResolveImportsForFile(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"src/index.ts":      "import { util } from './util';",
		"src/util.ts":       "export const util = 1;",
		"src/helper/lib.ts": "export const lib = 2;",
		"package.json":      `{"name": "test"}`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	factory := NewResolverFactory(tmpDir)

	// Test resolving imports for TypeScript file
	imports := []string{"./util", "./helper/lib", "react"}
	filePath := filepath.Join(tmpDir, "src/index.ts")

	resolved := factory.ResolveImportsForFile(filePath, model.LanguageTypeScript, imports)

	// Check that relative imports were resolved
	if resolved["./util"] != filepath.Join(tmpDir, "src/util.ts") {
		t.Errorf("Expected ./util to resolve to src/util.ts, got %s", resolved["./util"])
	}

	if resolved["./helper/lib"] != filepath.Join(tmpDir, "src/helper/lib.ts") {
		t.Errorf("Expected ./helper/lib to resolve to src/helper/lib.ts, got %s", resolved["./helper/lib"])
	}

	// react should not be in resolved map (node_modules not supported)
	if _, ok := resolved["react"]; ok {
		t.Errorf("Expected react to not be resolved, but it was: %s", resolved["react"])
	}
}

func TestFindProjectRoot(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create project structure
	projectStructure := []string{
		"package.json",
		"src/components/App.tsx",
		"src/utils/helper.ts",
	}

	for _, path := range projectStructure {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "file in src/components",
			filePath: filepath.Join(tmpDir, "src/components/App.tsx"),
			want:     tmpDir,
		},
		{
			name:     "file in src/utils",
			filePath: filepath.Join(tmpDir, "src/utils/helper.ts"),
			want:     tmpDir,
		},
		{
			name:     "file at root",
			filePath: filepath.Join(tmpDir, "package.json"),
			want:     tmpDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindProjectRoot(tt.filePath)
			if got != tt.want {
				t.Errorf("FindProjectRoot() = %v, want %v", got, tt.want)
			}
		})
	}
}
