package cobra

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSSAExtractor_NewAndEnable_NotPanic verifies SSAExtractor can be constructed and enabled without panic
func TestSSAExtractor_NewAndEnable_NotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewSSAExtractor or SetEnabled panicked: %v", r)
		}
	}()

	extractor := NewSSAExtractor()
	if extractor == nil {
		t.Fatal("NewSSAExtractor returned nil")
	}

	if extractor.IsEnabled() {
		t.Error("SSAExtractor should be disabled by default")
	}

	extractor.SetEnabled(true)
	if !extractor.IsEnabled() {
		t.Error("SetEnabled(true) did not enable SSAExtractor")
	}

	extractor.SetEnabled(false)
	if extractor.IsEnabled() {
		t.Error("SetEnabled(false) did not disable SSAExtractor")
	}
}

// TestSSAExtractor_ExtractFromPackage_EmptyDir_ReturnsNilOrEmpty verifies no panic on empty directory
func TestSSAExtractor_ExtractFromPackage_EmptyDir_ReturnsNilOrEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ExtractFromPackage panicked on empty dir: %v", r)
		}
	}()

	extractor := NewSSAExtractor()
	extractor.SetEnabled(true)

	// Create empty temp directory
	tmpDir := t.TempDir()

	contracts, err := extractor.ExtractFromPackage(tmpDir)
	if err != nil {
		// Errors are acceptable for empty directories
		t.Logf("ExtractFromPackage returned error (acceptable): %v", err)
	}

	// Should return nil or empty slice, not panic
	if contracts == nil {
		t.Log("ExtractFromPackage returned nil (acceptable)")
	} else if len(contracts) == 0 {
		t.Log("ExtractFromPackage returned empty slice (acceptable)")
	} else {
		t.Errorf("ExtractFromPackage returned %d contracts for empty directory", len(contracts))
	}
}

// TestSSAExtractor_ExtractFromPackage_NonGoModule_ReturnsNil verifies nil return for non-module paths
func TestSSAExtractor_ExtractFromPackage_NonGoModule_ReturnsNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ExtractFromPackage panicked on non-module dir: %v", r)
		}
	}()

	extractor := NewSSAExtractor()
	extractor.SetEnabled(true)

	// Create a temporary directory outside any Go module
	tmpDir := filepath.Join(os.TempDir(), "cobra-test-non-module")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a simple Go file (not in a module)
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package test

func Hello() string {
	return "hello"
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	contracts, err := extractor.ExtractFromPackage(tmpDir)

	// Should return nil (not in a module)
	if contracts != nil {
		t.Errorf("Expected nil for non-module path, got %d contracts", len(contracts))
	}
	if err != nil {
		t.Logf("ExtractFromPackage returned error (acceptable for non-module): %v", err)
	}
}

// TestSSAExtractor_ExtractFromPackage_Disabled_ReturnsNil verifies disabled extractor returns nil
func TestSSAExtractor_ExtractFromPackage_Disabled_ReturnsNil(t *testing.T) {
	extractor := NewSSAExtractor()
	// Keep disabled (default state)

	tmpDir := t.TempDir()

	contracts, err := extractor.ExtractFromPackage(tmpDir)

	if contracts != nil {
		t.Errorf("Disabled extractor should return nil, got %d contracts", len(contracts))
	}
	if err != nil {
		t.Errorf("Disabled extractor should not return error, got: %v", err)
	}
}
