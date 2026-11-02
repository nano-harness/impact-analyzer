package testutil

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func AssertReachInRange(t testing.TB, got, min, max int, msg string) {
	t.Helper()
	if got < min || got > max {
		t.Fatalf("%s: reach=%d, want in [%d,%d]", msg, got, min, max)
	}
}

func AssertRiskLevelOneOf(t testing.TB, got model.RiskLevel, allowed []model.RiskLevel) {
	t.Helper()
	for _, level := range allowed {
		if got == level {
			return
		}
	}
	t.Fatalf("risk level %q not in allowed set %v", got, allowed)
}

func AssertNoUnexpectedSymbols(t testing.TB, got []*model.Change, forbidden []string) {
	t.Helper()
	blocked := map[string]bool{}
	for _, name := range forbidden {
		blocked[name] = true
	}
	for _, change := range got {
		if change.Symbol != nil && blocked[change.Symbol.QualifiedName()] {
			t.Fatalf("unexpected symbol %s in changes", change.Symbol.QualifiedName())
		}
	}
}
