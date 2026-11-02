package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestSurfaceReporters(t *testing.T) {
	diff := &model.ContractDiff{ContractsAdded: []*model.Contract{{Kind: model.ContractKindHTTPRoute, Identifier: "GET /users", File: "api.go", Line: 10, Framework: "gin"}}}

	text, err := (&SurfaceTextReporter{}).GenerateSurface(diff)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "GET /users") {
		t.Fatalf("surface text missing contract: %s", text)
	}

	raw, err := (&SurfaceJSONReporter{}).GenerateSurface(diff)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schema_version"] != "1.0" {
		t.Fatalf("surface json schema=%v, want 1.0", decoded["schema_version"])
	}
}
