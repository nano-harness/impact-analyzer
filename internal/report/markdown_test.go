package report

import (
	"strings"
	"testing"
)

func TestMarkdownReporterFixedImpactResult(t *testing.T) {
	reporter, err := NewMarkdownReporter("")
	if err != nil {
		t.Fatal(err)
	}
	out, err := reporter.Generate(fixedImpactResult())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Impact Analysis", "Breaking Changes", "Changed Files", "Target"} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown output missing %q:\n%s", want, out)
		}
	}
}
