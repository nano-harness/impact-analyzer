package report

import (
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// Reporter defines the interface for generating reports
type Reporter interface {
	// Generate generates a report from impact results
	Generate(result *model.ImpactResult) (string, error)
}
