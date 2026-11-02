package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPosKeyOf verifies PosKey creation
func TestPosKeyOf(t *testing.T) {
	key := PosKeyOf("/path/to/file.go", 42, "MyFunc")
	if key.File != "/path/to/file.go" {
		t.Errorf("Expected file %q, got %q", "/path/to/file.go", key.File)
	}
	if key.Line != 42 {
		t.Errorf("Expected line 42, got %d", key.Line)
	}
	if key.Name != "MyFunc" {
		t.Errorf("Expected name %q, got %q", "MyFunc", key.Name)
	}
}

// TestPosKey_SameLineDifferentSymbols verifies that same-line declarations with different names don't collide
func TestPosKey_SameLineDifferentSymbols(t *testing.T) {
	// Simulate two short declarations on the same line (e.g., a, b := foo(), bar())
	key1 := PosKeyOf("/path/to/file.go", 10, "funcA")
	key2 := PosKeyOf("/path/to/file.go", 10, "funcB")

	reachable := map[PosKey]bool{
		key1: true,
	}

	if !reachable[key1] {
		t.Error("funcA should be reachable")
	}
	if reachable[key2] {
		t.Error("funcB should NOT be reachable — different name on same line")
	}

	// Also verify that Name="" falls back gracefully (legacy behavior)
	keyNoName := PosKeyOf("/path/to/file.go", 10, "")
	if reachable[keyNoName] {
		t.Error("empty-name key should not match named keys")
	}
}

// TestGoDeepReachability_NoGoMod verifies error when go.mod is missing
func TestGoDeepReachability_NoGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	provider := NewGoDeepReachability(tmpDir, nil, nil)
	reachable, err := provider.Reachable()

	if err == nil {
		t.Error("Expected error when go.mod is missing")
	}
	if reachable != nil {
		t.Error("Expected nil reachable set on error")
	}
}

// TestGoDeepReachability_WithMiniGoModule tests with a minimal Go module
func TestGoDeepReachability_WithMiniGoModule(t *testing.T) {
	// Use the current repository as test subject (it has go.mod)
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	// Verify go.mod exists
	goModPath := filepath.Join(repoRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		t.Skipf("Skipping test - no go.mod at %s", goModPath)
	}

	// Run reachability analysis on the actual repository
	provider := NewGoDeepReachability(repoRoot, nil, nil)
	reachable, err := provider.Reachable()

	if err != nil {
		t.Fatalf("Reachability analysis failed: %v", err)
	}

	if len(reachable) == 0 {
		t.Error("Expected some reachable functions in repository")
	}

	// Verify we got valid positions
	for pos := range reachable {
		if pos.File == "" {
			t.Error("Found position with empty file")
		}
		if pos.Line <= 0 {
			t.Errorf("Found position with invalid line: %d", pos.Line)
		}
	}

	t.Logf("Found %d reachable function positions", len(reachable))
}

// TestGoDeepReachability_WithBuildTags tests build tags support
func TestGoDeepReachability_WithBuildTags(t *testing.T) {
	t.Skip("Build tag testing requires complex test fixture setup")
}

// TestGoDeepReachability_WithCustomEntryPoints tests custom entry points
func TestGoDeepReachability_WithCustomEntryPoints(t *testing.T) {
	t.Skip("Custom entry point testing requires complex test fixture setup")
}

// TestGoDeepReachability_InterfaceImplementation tests that interface implementations are tracked
func TestGoDeepReachability_InterfaceImplementation(t *testing.T) {
	t.Skip("Interface implementation testing requires complex test fixture setup")
}
