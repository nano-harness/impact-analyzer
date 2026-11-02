package cobra

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadModuleSSA_FirstLoadAndCacheHit verifies caching works
func TestLoadModuleSSA_FirstLoadAndCacheHit(t *testing.T) {
	// Reset cache before test
	ResetSSACache()

	// Get paths to files in the same module (this repo)
	repoRoot, err := filepath.Abs("../../../../")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	file1 := filepath.Join(repoRoot, "cmd", "cr.go")
	file2 := filepath.Join(repoRoot, "cmd", "analyze.go")

	// Check files exist
	if _, err := os.Stat(file1); os.IsNotExist(err) {
		t.Skipf("File %s does not exist, skipping test", file1)
	}
	if _, err := os.Stat(file2); os.IsNotExist(err) {
		t.Skipf("File %s does not exist, skipping test", file2)
	}

	// First load (cache miss)
	t1 := time.Now()
	prog1, moduleRoot1, err := LoadModuleSSA(file1)
	d1 := time.Since(t1)
	if err != nil {
		t.Fatalf("First LoadModuleSSA failed: %v", err)
	}
	if prog1 == nil {
		t.Fatal("First LoadModuleSSA returned nil prog")
	}
	if moduleRoot1 == "" {
		t.Fatal("First LoadModuleSSA returned empty moduleRoot")
	}

	t.Logf("First load took %v", d1)

	// Second load (cache hit, same module)
	t2 := time.Now()
	prog2, moduleRoot2, err := LoadModuleSSA(file2)
	d2 := time.Since(t2)
	if err != nil {
		t.Fatalf("Second LoadModuleSSA failed: %v", err)
	}
	if prog2 == nil {
		t.Fatal("Second LoadModuleSSA returned nil prog")
	}
	if moduleRoot2 == "" {
		t.Fatal("Second LoadModuleSSA returned empty moduleRoot")
	}

	t.Logf("Second load (cache hit) took %v", d2)

	// Verify cache hit: same prog instance
	if prog1 != prog2 {
		t.Error("Expected same *ssa.Program instance from cache, got different instances")
	}

	// Verify same module root
	if moduleRoot1 != moduleRoot2 {
		t.Errorf("Expected same moduleRoot, got %s and %s", moduleRoot1, moduleRoot2)
	}

	// Verify cache hit is fast (< 200ms)
	if d2 > 200*time.Millisecond {
		t.Errorf("Cache hit too slow: %v (expected < 200ms)", d2)
	}
}

// TestLoadModuleSSA_TempDirSkipsCache verifies temp dir fallback doesn't cache
func TestLoadModuleSSA_TempDirSkipsCache(t *testing.T) {
	// Reset cache before test
	ResetSSACache()

	// Get cache size before
	globalSSACache.mu.RLock()
	sizeBefore := len(globalSSACache.entries)
	globalSSACache.mu.RUnlock()

	// Create a temp dir with minimal Go file
	tmpDir, err := os.MkdirTemp("", "cobra-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a minimal valid Go file
	tmpFile := filepath.Join(tmpDir, "main.go")
	content := `package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{Use: "test"}

func main() {
	rootCmd.Execute()
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// First load (should not cache because it's in temp dir)
	prog1, _, err := LoadModuleSSA(tmpFile)
	// Note: This may fail if temp dir is not a valid Go module, which is expected
	if err != nil {
		t.Logf("LoadModuleSSA failed (expected for temp dir): %v", err)
	} else if prog1 != nil {
		t.Logf("LoadModuleSSA succeeded with single-package fallback")
	}

	// Second load
	prog2, _, err := LoadModuleSSA(tmpFile)
	if err != nil {
		t.Logf("LoadModuleSSA failed again (expected for temp dir): %v", err)
	} else if prog2 != nil {
		t.Logf("LoadModuleSSA succeeded again with single-package fallback")
	}

	// Verify cache size didn't change
	globalSSACache.mu.RLock()
	sizeAfter := len(globalSSACache.entries)
	globalSSACache.mu.RUnlock()

	if sizeAfter != sizeBefore {
		t.Errorf("Cache size changed from %d to %d, expected no change for temp dir", sizeBefore, sizeAfter)
	}
}

// TestResetSSACache_ClearsEntries verifies cache reset works
func TestResetSSACache_ClearsEntries(t *testing.T) {
	// Reset cache before test
	ResetSSACache()

	// Load once to populate cache
	repoRoot, err := filepath.Abs("../../../../")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	file1 := filepath.Join(repoRoot, "cmd", "cr.go")
	if _, err := os.Stat(file1); os.IsNotExist(err) {
		t.Skipf("File %s does not exist, skipping test", file1)
	}

	_, _, err = LoadModuleSSA(file1)
	if err != nil {
		t.Fatalf("LoadModuleSSA failed: %v", err)
	}

	// Verify cache is non-empty
	globalSSACache.mu.RLock()
	sizeBefore := len(globalSSACache.entries)
	globalSSACache.mu.RUnlock()

	if sizeBefore == 0 {
		t.Fatal("Cache should be non-empty after LoadModuleSSA")
	}

	// Reset cache
	ResetSSACache()

	// Verify cache is empty
	globalSSACache.mu.RLock()
	sizeAfter := len(globalSSACache.entries)
	globalSSACache.mu.RUnlock()

	if sizeAfter != 0 {
		t.Errorf("Cache size is %d after reset, expected 0", sizeAfter)
	}
}

// TestLoadModuleSSA_NotInGoModule_ReturnsError verifies error handling
func TestLoadModuleSSA_NotInGoModule_ReturnsError(t *testing.T) {
	// Try to load from root directory (not a Go module)
	prog, moduleRoot, err := LoadModuleSSA("/")

	// Should return an error
	if err == nil {
		t.Error("Expected error for non-Go-module path, got nil")
	}

	if prog != nil {
		t.Error("Expected nil prog for non-Go-module path, got non-nil")
	}

	if moduleRoot != "" {
		t.Errorf("Expected empty moduleRoot for non-Go-module path, got %s", moduleRoot)
	}

	t.Logf("Got expected error: %v", err)
}
