package analysis

import (
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestCoverageAnnotation(t *testing.T) {
	profile, err := LoadCoverageProfile(filepath.Join("..", "testutil", "testdata", "coverage", "sample.coverage.txt"))
	if err != nil {
		t.Fatal(err)
	}

	covered := &model.Symbol{File: "github.com/nano-harness/impact-analyzer/sample.go", Line: 3, EndLine: 5}
	partial := &model.Symbol{File: "github.com/nano-harness/impact-analyzer/sample.go", Line: 4, EndLine: 7}
	missing := &model.Symbol{File: "missing.go", Line: 1, EndLine: 2}

	AnnotateSymbol(covered, profile)
	AnnotateSymbol(partial, profile)
	AnnotateSymbol(missing, profile)

	if covered.CoverageStatus != model.CoverageStatusCovered {
		t.Fatalf("covered status=%q, want covered", covered.CoverageStatus)
	}
	if partial.CoverageStatus != model.CoverageStatusPartial {
		t.Fatalf("partial status=%q, want partial", partial.CoverageStatus)
	}
	if missing.CoverageStatus != model.CoverageStatusUncovered {
		t.Fatalf("missing status=%q, want uncovered", missing.CoverageStatus)
	}
}
