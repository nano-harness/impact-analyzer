package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJavaResolver_ResolveImport(t *testing.T) {
	// Create a temporary test directory structure
	tmpDir := t.TempDir()

	// Create standard Maven/Gradle directory structure
	testFiles := map[string]string{
		"src/main/java/com/example/Service.java":     "public class Service {}",
		"src/main/java/com/example/util/Helper.java": "public class Helper {}",
		"src/test/java/com/example/ServiceTest.java": "public class ServiceTest {}",
		"src/main/java/org/another/Component.java":   "public class Component {}",
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

	resolver := NewJavaResolver(tmpDir)

	tests := []struct {
		name       string
		importPath string
		fromFile   string
		want       string
		wantErr    bool
	}{
		{
			name:       "resolve class in main source root",
			importPath: "com.example.Service",
			fromFile:   filepath.Join(tmpDir, "src/main/java/com/example/Main.java"),
			want:       filepath.Join(tmpDir, "src/main/java/com/example/Service.java"),
			wantErr:    false,
		},
		{
			name:       "resolve class in nested package",
			importPath: "com.example.util.Helper",
			fromFile:   filepath.Join(tmpDir, "src/main/java/com/example/Service.java"),
			want:       filepath.Join(tmpDir, "src/main/java/com/example/util/Helper.java"),
			wantErr:    false,
		},
		{
			name:       "resolve test class",
			importPath: "com.example.ServiceTest",
			fromFile:   filepath.Join(tmpDir, "src/test/java/com/example/AnotherTest.java"),
			want:       filepath.Join(tmpDir, "src/test/java/com/example/ServiceTest.java"),
			wantErr:    false,
		},
		{
			name:       "resolve class in different package",
			importPath: "org.another.Component",
			fromFile:   filepath.Join(tmpDir, "src/main/java/com/example/Service.java"),
			want:       filepath.Join(tmpDir, "src/main/java/org/another/Component.java"),
			wantErr:    false,
		},
		{
			name:       "non-existent class should fail",
			importPath: "com.example.NonExistent",
			fromFile:   filepath.Join(tmpDir, "src/main/java/com/example/Service.java"),
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

func TestJavaResolver_WithSourceRoots(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a custom source structure
	customPath := "custom/src/com/example/Custom.java"
	fullPath := filepath.Join(tmpDir, customPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("public class Custom {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create resolver with custom source roots
	resolver := NewJavaResolver(tmpDir).WithSourceRoots([]string{"custom/src"})

	got, err := resolver.ResolveImport("com.example.Custom", "")
	if err != nil {
		t.Errorf("ResolveImport() with custom source roots failed: %v", err)
		return
	}

	want := filepath.Join(tmpDir, "custom/src/com/example/Custom.java")
	if got != want {
		t.Errorf("ResolveImport() = %v, want %v", got, want)
	}
}
