package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSResolver_ResolveImport(t *testing.T) {
	// Create a temporary test directory structure
	tmpDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"src/index.ts":            "export const main = () => {}",
		"src/helpers/index.ts":    "export const utilIndex = () => {}",
		"src/helpers/helper.js":   "export const helper = () => {}",
		"src/components/App.tsx":  "export const App = () => {}",
		"src/components/utils.ts": "export const componentUtil = () => {}",
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

	resolver := NewJSResolver(tmpDir)

	tests := []struct {
		name       string
		importPath string
		fromFile   string
		want       string
		wantErr    bool
	}{
		{
			name:       "resolve relative .ts file",
			importPath: "./utils",
			fromFile:   filepath.Join(tmpDir, "src/components/App.tsx"),
			want:       filepath.Join(tmpDir, "src/components/utils.ts"),
			wantErr:    false,
		},
		{
			name:       "resolve directory with index file",
			importPath: "../helpers",
			fromFile:   filepath.Join(tmpDir, "src/components/App.tsx"),
			want:       filepath.Join(tmpDir, "src/helpers/index.ts"),
			wantErr:    false,
		},
		{
			name:       "resolve parent directory file",
			importPath: "../index",
			fromFile:   filepath.Join(tmpDir, "src/components/App.tsx"),
			want:       filepath.Join(tmpDir, "src/index.ts"),
			wantErr:    false,
		},
		{
			name:       "resolve .js file from .tsx file",
			importPath: "../helpers/helper",
			fromFile:   filepath.Join(tmpDir, "src/components/App.tsx"),
			want:       filepath.Join(tmpDir, "src/helpers/helper.js"),
			wantErr:    false,
		},
		{
			name:       "non-relative import should fail",
			importPath: "react",
			fromFile:   filepath.Join(tmpDir, "src/index.ts"),
			want:       "",
			wantErr:    true,
		},
		{
			name:       "non-existent file should fail",
			importPath: "./nonexistent",
			fromFile:   filepath.Join(tmpDir, "src/index.ts"),
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.ResolveImport(tt.importPath, tt.fromFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveImport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveImport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJSResolver_findJSFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files with different extensions
	testFiles := []string{
		"file.ts",
		"file2.tsx",
		"file3.js",
		"dir/index.ts",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	resolver := NewJSResolver(tmpDir)

	tests := []struct {
		name     string
		basePath string
		want     string
		wantErr  bool
	}{
		{
			name:     "find .ts file",
			basePath: filepath.Join(tmpDir, "file"),
			want:     filepath.Join(tmpDir, "file.ts"),
			wantErr:  false,
		},
		{
			name:     "find .tsx file",
			basePath: filepath.Join(tmpDir, "file2"),
			want:     filepath.Join(tmpDir, "file2.tsx"),
			wantErr:  false,
		},
		{
			name:     "find .js file",
			basePath: filepath.Join(tmpDir, "file3"),
			want:     filepath.Join(tmpDir, "file3.js"),
			wantErr:  false,
		},
		{
			name:     "find index file in directory",
			basePath: filepath.Join(tmpDir, "dir"),
			want:     filepath.Join(tmpDir, "dir/index.ts"),
			wantErr:  false,
		},
		{
			name:     "non-existent file",
			basePath: filepath.Join(tmpDir, "nonexistent"),
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.findJSFile(tt.basePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("findJSFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("findJSFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
