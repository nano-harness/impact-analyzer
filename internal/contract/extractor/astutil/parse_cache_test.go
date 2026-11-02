package astutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseContentParsesOncePerFile verifies that repeated ParseContent calls
// for the same unchanged file (as several same-language extractors do via
// their ExtractContent) share a single tree-sitter parse, and that
// ClearParseCache ends the cache lifecycle explicitly.
func TestParseContentParsesOncePerFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.ts")
	content := []byte("import express from 'express';\nconst app = express();\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	defer ClearParseCache()

	// First call reads from disk; second call passes pre-read content like
	// ExtractContent does. Both must observe exactly one real parse.
	_, tree1, got1, _, err := ParseContent(path, nil)
	if err != nil {
		t.Fatalf("first ParseContent failed: %v", err)
	}
	_, tree2, got2, _, err := ParseContent(path, content)
	if err != nil {
		t.Fatalf("second ParseContent failed: %v", err)
	}
	if tree1 != tree2 {
		t.Fatal("second ParseContent returned a different tree, want the shared cached tree")
	}
	if string(got1) != string(content) || string(got2) != string(content) {
		t.Fatalf("cached content mismatch: got %q / %q, want %q", got1, got2, content)
	}
	if n := RealParseCount(); n != 1 {
		t.Fatalf("RealParseCount = %d, want 1 (two lookups, one parse)", n)
	}

	// ClearParseCache closes the cached trees and resets the counter; the
	// next lookup parses again.
	ClearParseCache()
	if n := RealParseCount(); n != 0 {
		t.Fatalf("RealParseCount after ClearParseCache = %d, want 0", n)
	}
	_, tree3, _, _, err := ParseContent(path, content)
	if err != nil {
		t.Fatalf("ParseContent after ClearParseCache failed: %v", err)
	}
	if tree3 == tree1 {
		t.Fatal("ParseContent after ClearParseCache returned the closed tree, want a fresh parse")
	}
	if n := RealParseCount(); n != 1 {
		t.Fatalf("RealParseCount after re-parse = %d, want 1", n)
	}
}

// TestParseCacheInvalidatesOnFileChange verifies the (path, mtime, size) key
// re-parses a file that changed on disk instead of serving a stale tree.
func TestParseCacheInvalidatesOnFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.ts")
	if err := os.WriteFile(path, []byte("const a = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer ClearParseCache()

	if _, _, _, _, err := ParseContent(path, nil); err != nil {
		t.Fatalf("first ParseContent failed: %v", err)
	}
	if n := RealParseCount(); n != 1 {
		t.Fatalf("RealParseCount = %d, want 1", n)
	}

	// Rewrite with different content (different size => different cache key).
	if err := os.WriteFile(path, []byte("const a = 1;\nconst b = 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := ParseContent(path, nil); err != nil {
		t.Fatalf("ParseContent after rewrite failed: %v", err)
	}
	if n := RealParseCount(); n != 2 {
		t.Fatalf("RealParseCount after rewrite = %d, want 2 (changed file re-parsed)", n)
	}
}
