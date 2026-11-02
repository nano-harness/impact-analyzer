package report

import (
	"encoding/json"
	"testing"
)

func TestJSONReporterContainsKeyFields(t *testing.T) {
	out, err := NewJSONReporter().Generate(fixedImpactResult())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "changes", "total_reach", "breaking_count"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("JSON output missing key %q: %s", key, out)
		}
	}
}
