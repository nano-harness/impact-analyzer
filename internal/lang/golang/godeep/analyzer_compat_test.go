package godeep

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAnalyzer_LoadPackages_ToleratesBrokenSibling verifies that a package
// with type errors elsewhere in the load graph (e.g. code requiring a newer
// Go toolchain than the one running this binary) does not abort SSA loading
// for the packages that are fine, as long as at least one root package
// produced usable syntax.
func TestAnalyzer_LoadPackages_ToleratesBrokenSibling(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("go.mod", "module example.com/ssacompat\n\ngo 1.22\n")
	writeFile("good/good.go", "package good\n\n// Hello returns a greeting.\nfunc Hello() string { return \"hi\" }\n")
	writeFile("bad/bad.go", "package bad\n\n// Broken references an undefined symbol.\nfunc Broken() int { return undefinedSymbol }\n")

	analyzer := NewAnalyzer()
	if err := analyzer.LoadPackages(dir + "/..."); err != nil {
		t.Fatalf("LoadPackages should tolerate a broken sibling package, got: %v", err)
	}
	if len(analyzer.pkgs) == 0 {
		t.Fatal("expected at least one loaded package")
	}

	if err := analyzer.BuildSSA(); err != nil {
		t.Fatalf("BuildSSA should succeed with partial load, got: %v", err)
	}
	if analyzer.Prog == nil {
		t.Fatal("expected SSA program to be built")
	}
}

// TestAnalyzer_LoadPackages_AllBrokenFails verifies that loading still fails
// — with the underlying error message — when no root package is usable.
func TestAnalyzer_LoadPackages_AllBrokenFails(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("go.mod", "module example.com/ssacompat\n\ngo 1.22\n")
	writeFile("bad/bad.go", "package bad\n\nfunc Broken() int { return undefinedSymbol }\n")

	analyzer := NewAnalyzer()
	err := analyzer.LoadPackages(dir + "/...")
	if err == nil {
		t.Fatal("expected LoadPackages to fail when no root package is usable")
	}
	if got := err.Error(); got == "errors loading packages" {
		t.Fatalf("expected the underlying error message to be surfaced, got opaque: %q", got)
	}
}
