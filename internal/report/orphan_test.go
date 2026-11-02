package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestOrphanReport_TopN_Groups(t *testing.T) {
	result := &model.OrphanResult{}
	for i := 0; i < 7; i++ {
		result.Candidates = append(result.Candidates, &model.OrphanCandidate{
			Symbol:     &model.Symbol{Name: fmt.Sprintf("Func%d", i), Kind: model.SymbolKindFunction, File: "pkg/service.go", Line: i + 1},
			Confidence: model.ConfidenceMedium,
			Score:      float64(10 - i),
			Reasons:    []string{"not reachable"},
		})
	}
	result.Candidates = append(result.Candidates, &model.OrphanCandidate{
		Symbol:     &model.Symbol{Name: "SomeInterface", Kind: model.SymbolKindInterface, File: "pkg/api.go", Line: 20},
		Confidence: model.ConfidenceHigh,
		Score:      20,
		Reasons:    []string{"not reachable"},
	})
	result.Candidates = append(result.Candidates, &model.OrphanCandidate{
		Symbol:     &model.Symbol{Name: "localVar", Kind: model.SymbolKindVariable, File: "pkg/service.go", Line: 30},
		Confidence: model.ConfidenceLow,
		Score:      1,
		Reasons:    []string{"not reachable"},
	})
	result.Summary.Total = len(result.Candidates)

	out, err := GenerateOrphanReportWithOptions(result, "markdown", OrphanReportOptions{Limit: 50, GroupBy: "kind"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Top 10 高优先级候选", "`SomeInterface`", "### function", "...and 2 more (use --all to view)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown report missing %q:\n%s", want, out)
		}
	}
}

func TestOrphanReport_JSONIsCompleteDespiteLimit(t *testing.T) {
	result := &model.OrphanResult{}
	for i := 0; i < 3; i++ {
		result.Candidates = append(result.Candidates, &model.OrphanCandidate{
			Symbol:     &model.Symbol{Name: fmt.Sprintf("Func%d", i), Kind: model.SymbolKindFunction, File: "pkg/service.go", Line: i + 1},
			Confidence: model.ConfidenceMedium,
			Score:      float64(i),
		})
	}
	out, err := GenerateOrphanReportWithOptions(result, "json", OrphanReportOptions{Limit: 1, GroupBy: "kind"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded model.OrphanResult
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Candidates) != 3 {
		t.Fatalf("json candidates = %d; want full 3", len(decoded.Candidates))
	}
}
