package index

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
)

func TestManagerLifecycleCreatesCacheAndRejectsUnknownFile(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	mgr, err := NewManager(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	if _, err := os.Stat(filepath.Join(cacheDir, "index.db")); err != nil {
		t.Fatalf("index db not created: %v", err)
	}
	if err := mgr.IndexFile(filepath.Join(cacheDir, "unknown.txt")); err == nil {
		t.Fatal("IndexFile unknown language succeeded, want error")
	}
}

func TestManagerIncrementalBuildCaching(t *testing.T) {
	// #15: Test file-level caching
	cacheDir := filepath.Join(t.TempDir(), "cache")
	mgr, err := NewManager(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Create a test Go file
	testFile := filepath.Join(t.TempDir(), "test.go")
	content := `package main
func hello() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Index the file first time
	if err := mgr.IndexFile(testFile); err != nil {
		t.Fatal(err)
	}

	// Get cached symbols
	fs1, err := mgr.store.GetFileSymbols(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if fs1.ContentHash == "" {
		t.Fatal("expected ContentHash to be set")
	}

	// Index again without modifying - should use cache
	if err := mgr.IndexFile(testFile); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	modifiedContent := `package main
func hello() {}
func world() {}
`
	if err := os.WriteFile(testFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Index modified file - should re-parse
	if err := mgr.IndexFile(testFile); err != nil {
		t.Fatal(err)
	}

	// Get new cached symbols
	fs2, err := mgr.store.GetFileSymbols(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// ContentHash should be different
	if fs1.ContentHash == fs2.ContentHash {
		t.Fatal("expected ContentHash to change after file modification")
	}

	// New file should have more symbols
	if len(fs2.Symbols) <= len(fs1.Symbols) {
		t.Fatalf("expected more symbols after adding function, got %d vs %d", len(fs2.Symbols), len(fs1.Symbols))
	}
}
