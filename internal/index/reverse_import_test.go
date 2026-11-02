package index

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestReverseImportIndex_AddFileImports(t *testing.T) {
	idx := NewReverseImportIndex()

	idx.AddFileImports("main.go", []string{"fmt", "strings"})
	idx.AddFileImports("utils.go", []string{"fmt", "os"})

	// Check forward index
	imports := idx.GetImports("main.go")
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports for main.go, got %d", len(imports))
	}

	// Check reverse index
	fmtImporters := idx.GetImporters("fmt")
	if len(fmtImporters) != 2 {
		t.Errorf("Expected 2 importers of fmt, got %d", len(fmtImporters))
	}

	osImporters := idx.GetImporters("os")
	if len(osImporters) != 1 {
		t.Errorf("Expected 1 importer of os, got %d", len(osImporters))
	}
}

func TestReverseImportIndex_RemoveFile(t *testing.T) {
	idx := NewReverseImportIndex()

	idx.AddFileImports("main.go", []string{"fmt", "strings"})
	idx.AddFileImports("utils.go", []string{"fmt", "os"})

	// Remove main.go
	idx.RemoveFile("main.go")

	// Check main.go is gone
	imports := idx.GetImports("main.go")
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports for main.go after removal, got %d", len(imports))
	}

	// Check fmt now has only 1 importer
	fmtImporters := idx.GetImporters("fmt")
	if len(fmtImporters) != 1 {
		t.Errorf("Expected 1 importer of fmt after removal, got %d", len(fmtImporters))
	}

	// Check strings has no importers
	stringsImporters := idx.GetImporters("strings")
	if len(stringsImporters) != 0 {
		t.Errorf("Expected 0 importers of strings after removal, got %d", len(stringsImporters))
	}
}

func TestReverseImportIndex_GetImporters(t *testing.T) {
	idx := NewReverseImportIndex()

	idx.AddFileImports("main.go", []string{"fmt"})
	idx.AddFileImports("utils.go", []string{"fmt"})
	idx.AddFileImports("helpers.go", []string{"strings"})

	tests := []struct {
		module   string
		expected int
	}{
		{"fmt", 2},
		{"strings", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		importers := idx.GetImporters(tt.module)
		if len(importers) != tt.expected {
			t.Errorf("GetImporters(%q) returned %d files, expected %d", tt.module, len(importers), tt.expected)
		}
	}
}

func TestReverseImportIndex_Stats(t *testing.T) {
	idx := NewReverseImportIndex()

	idx.AddFileImports("main.go", []string{"fmt", "strings"})
	idx.AddFileImports("utils.go", []string{"fmt", "os"})

	fileCount, moduleCount := idx.Stats()

	if fileCount != 2 {
		t.Errorf("Expected 2 files, got %d", fileCount)
	}

	if moduleCount != 3 {
		t.Errorf("Expected 3 modules (fmt, strings, os), got %d", moduleCount)
	}
}

func TestReverseImportIndex_Clear(t *testing.T) {
	idx := NewReverseImportIndex()

	idx.AddFileImports("main.go", []string{"fmt", "strings"})
	idx.AddFileImports("utils.go", []string{"fmt", "os"})

	idx.Clear()

	fileCount, moduleCount := idx.Stats()
	if fileCount != 0 || moduleCount != 0 {
		t.Errorf("After Clear(), expected 0 files and 0 modules, got %d files and %d modules", fileCount, moduleCount)
	}
}

func TestReverseImportIndex_BuildFromFileSymbols(t *testing.T) {
	idx := NewReverseImportIndex()

	files := []*model.FileSymbols{
		{
			File:    "main.go",
			Imports: []string{"fmt", "strings"},
		},
		{
			File:    "utils.go",
			Imports: []string{"fmt", "os"},
		},
	}

	idx.BuildFromFileSymbols(files)

	fileCount, moduleCount := idx.Stats()
	if fileCount != 2 {
		t.Errorf("Expected 2 files, got %d", fileCount)
	}

	if moduleCount != 3 {
		t.Errorf("Expected 3 modules, got %d", moduleCount)
	}

	fmtImporters := idx.GetImporters("fmt")
	if len(fmtImporters) != 2 {
		t.Errorf("Expected 2 importers of fmt, got %d", len(fmtImporters))
	}
}

func TestReverseImportIndex_Concurrent(t *testing.T) {
	idx := NewReverseImportIndex()

	// Add some initial data
	idx.AddFileImports("main.go", []string{"fmt"})

	// Concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			idx.AddFileImports("file.go", []string{"pkg1", "pkg2"})
			idx.RemoveFile("file.go")
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = idx.GetImporters("fmt")
			_ = idx.GetImports("main.go")
			_, _ = idx.Stats()
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	// Verify index is still consistent
	fmtImporters := idx.GetImporters("fmt")
	if len(fmtImporters) != 1 {
		t.Errorf("Expected 1 importer of fmt after concurrent operations, got %d", len(fmtImporters))
	}
}

func TestImportPackagePrefix(t *testing.T) {
	tests := []struct {
		imp  string
		want string
	}{
		{"com.example.lib.Helper", "com.example.lib"}, // Java class import
		{"models.User", "models"},                     // Python class import
		{"com.example.lib", ""},                       // package-only import (lowercase tail)
		{"lib.greet", ""},                             // Python member import
		{"os.path", ""},                               // Python dotted module
		{"example.com/eval/lib", ""},                  // Go module path
		{"github.com/Foo/Bar", ""},                    // slash-bearing: never stripped
		{"./lib", ""},                                 // TS relative specifier
		{"fmt", ""},                                   // no dot
		{"com.example.lib.*", ""},                     // Java wildcard import
	}
	for _, tt := range tests {
		if got := importPackagePrefix(tt.imp); got != tt.want {
			t.Errorf("importPackagePrefix(%q) = %q, want %q", tt.imp, got, tt.want)
		}
	}
}

func TestReverseImportIndex_PackagePrefixKeys(t *testing.T) {
	idx := NewReverseImportIndex()

	// Java-style class import: also filed under the package prefix.
	idx.AddFileImports("Service.java", []string{"com.example.lib.Helper"})
	// Two class imports from the same package must not duplicate the file
	// under the derived prefix key.
	idx.AddFileImports("Both.java", []string{"com.example.lib.Helper", "com.example.lib.Util"})
	// Other languages' import shapes must not gain prefix keys.
	idx.AddFileImports("svc.go", []string{"example.com/eval/lib"})
	idx.AddFileImports("svc.ts", []string{"./lib"})
	idx.AddFileImports("svc.py", []string{"lib.greet"})

	if got := idx.GetImporters("com.example.lib"); len(got) != 2 {
		t.Errorf("Expected 2 importers of derived prefix com.example.lib, got %v", got)
	}
	if got := idx.GetImporters("com.example.lib.Helper"); len(got) != 2 {
		t.Errorf("Expected 2 importers of exact key com.example.lib.Helper, got %v", got)
	}
	for _, key := range []string{"example.com/eval", ".", "lib"} {
		if got := idx.GetImporters(key); len(got) != 0 {
			t.Errorf("Expected no importers under spurious prefix key %q, got %v", key, got)
		}
	}

	_, moduleCount := idx.Stats()
	// 5 exact keys + 1 derived prefix key.
	if moduleCount != 6 {
		t.Errorf("Expected 6 module keys (5 exact + 1 derived), got %d", moduleCount)
	}
}

func TestReverseImportIndex_RemoveFileDropsDerivedPrefix(t *testing.T) {
	idx := NewReverseImportIndex()
	idx.AddFileImports("Service.java", []string{"com.example.lib.Helper", "com.example.lib.Util"})
	idx.AddFileImports("Other.java", []string{"com.example.lib.Helper"})

	idx.RemoveFile("Service.java")

	got := idx.GetImporters("com.example.lib")
	if len(got) != 1 || got[0] != "Other.java" {
		t.Errorf("Expected only Other.java under derived key com.example.lib, got %v", got)
	}

	idx.RemoveFile("Other.java")
	if got := idx.GetImporters("com.example.lib"); len(got) != 0 {
		t.Errorf("Expected derived key com.example.lib to be empty, got %v", got)
	}
	if got := idx.GetImporters("com.example.lib.Helper"); len(got) != 0 {
		t.Errorf("Expected exact key com.example.lib.Helper to be empty, got %v", got)
	}
}
