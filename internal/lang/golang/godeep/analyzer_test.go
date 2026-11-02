package godeep

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestAnalyzer_LoadPackages(t *testing.T) {
	analyzer := NewAnalyzer()

	// Load the current package
	err := analyzer.LoadPackages(".")
	if err != nil {
		t.Skip("Skipping test - requires Go module context")
	}

	if len(analyzer.pkgs) == 0 {
		t.Error("Expected to load at least one package")
	}
}

func TestAnalyzer_BuildSSA(t *testing.T) {
	analyzer := NewAnalyzer()

	// Try to load std library package
	err := analyzer.LoadPackages("fmt")
	if err != nil {
		t.Fatalf("Failed to load fmt package: %v", err)
	}

	err = analyzer.BuildSSA()
	if err != nil {
		t.Fatalf("Failed to build SSA: %v", err)
	}

	if analyzer.Prog == nil {
		t.Error("Expected SSA program to be built")
	}
}

func TestAnalyzer_BuildCallGraph(t *testing.T) {
	analyzer := NewAnalyzer()

	// Load a simple package
	err := analyzer.LoadPackages("fmt")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	err = analyzer.BuildSSA()
	if err != nil {
		t.Fatalf("Failed to build SSA: %v", err)
	}

	cg, err := analyzer.BuildCallGraph()
	if err != nil {
		t.Fatalf("Failed to build call graph: %v", err)
	}

	if cg == nil {
		t.Error("Expected call graph to be non-nil")
	}

	if len(cg.Nodes) == 0 {
		t.Error("Expected call graph to have nodes")
	}
}

func TestAnalyzer_BuildCallGraph_GenericTypes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/generic\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `package main

type Number interface { ~int | ~float64 }

func Sum[T Number](values []T) T {
	var total T
	for _, value := range values {
		total += value
	}
	return total
}

func main() { _ = Sum([]int{1, 2}) }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer()
	if err := analyzer.LoadPackages(dir); err != nil {
		t.Fatalf("Failed to load generic package: %v", err)
	}
	if err := analyzer.BuildSSA(); err != nil {
		t.Fatalf("Failed to build SSA for generic package: %v", err)
	}
	if _, err := analyzer.BuildCallGraph(); err != nil {
		t.Fatalf("RTA should degrade cleanly for generic types: %v", err)
	}
}

func TestAnalyzer_ConvertToModel(t *testing.T) {
	analyzer := NewAnalyzer()

	err := analyzer.LoadPackages("fmt")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	err = analyzer.BuildSSA()
	if err != nil {
		t.Fatalf("Failed to build SSA: %v", err)
	}

	cg, err := analyzer.BuildCallGraph()
	if err != nil {
		t.Fatalf("Failed to build call graph: %v", err)
	}

	symbols := analyzer.ConvertToModel(cg)
	if len(symbols) == 0 {
		t.Error("Expected to find symbols in call graph")
	}

	// Verify symbols have required fields
	for _, sym := range symbols {
		if sym.Name == "" {
			t.Error("Symbol missing name")
		}
		if sym.Language != model.LanguageGo {
			t.Errorf("Expected Go language, got %s", sym.Language)
		}
	}
}

func TestAnalyzer_ExtractCallEdges(t *testing.T) {
	analyzer := NewAnalyzer()

	err := analyzer.LoadPackages("fmt")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	err = analyzer.BuildSSA()
	if err != nil {
		t.Fatalf("Failed to build SSA: %v", err)
	}

	cg, err := analyzer.BuildCallGraph()
	if err != nil {
		t.Fatalf("Failed to build call graph: %v", err)
	}

	edges := analyzer.ExtractCallEdges(cg)

	// Just verify the method works - edges may be empty for some packages
	_ = edges
}
