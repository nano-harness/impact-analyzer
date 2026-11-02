package report

import (
	"strings"
	"testing"
)

func TestLLMContextReporterBasicOutput(t *testing.T) {
	out, err := NewLLMContextReporter(0).Generate(fixedImpactResult())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Code Change Impact Analysis", "Breaking Changes", "pkg.Target"} {
		if !strings.Contains(out, want) {
			t.Fatalf("llm context missing %q:\n%s", want, out)
		}
	}
}
