package report

import (
	"encoding/json"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// JSONReporter generates JSON reports
type JSONReporter struct{}

// NewJSONReporter creates a new JSON reporter
func NewJSONReporter() *JSONReporter {
	return &JSONReporter{}
}

// Generate generates a JSON report
func (jr *JSONReporter) Generate(result *model.ImpactResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
