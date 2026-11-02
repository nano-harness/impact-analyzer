package report

import (
	"fmt"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// MermaidReporter generates Mermaid graph diagrams
type MermaidReporter struct{}

// NewMermaidReporter creates a new Mermaid reporter
func NewMermaidReporter() *MermaidReporter {
	return &MermaidReporter{}
}

// Generate generates a Mermaid diagram
func (mr *MermaidReporter) Generate(result *model.ImpactResult) (string, error) {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("graph TD\n")

	// Generate nodes and edges for each change
	for i, change := range result.Changes {
		if change.Change.Symbol == nil {
			continue
		}

		changeNode := fmt.Sprintf("C%d", i)
		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>%s\"]\n",
			changeNode,
			change.Change.Symbol.Name,
			change.RiskLevel))

		// Add style based on risk level
		switch change.RiskLevel {
		case model.RiskLevelBreaking:
			sb.WriteString(fmt.Sprintf("    style %s fill:#ff6b6b\n", changeNode))
		case model.RiskLevelWarning:
			sb.WriteString(fmt.Sprintf("    style %s fill:#ffd93d\n", changeNode))
		case model.RiskLevelSafe:
			sb.WriteString(fmt.Sprintf("    style %s fill:#6bcf7f\n", changeNode))
		}

		// Add edges to callers (limit to first 10 to avoid clutter)
		maxCallers := 10
		for j, caller := range change.DirectCallers {
			if j >= maxCallers {
				break
			}
			callerNode := fmt.Sprintf("CA%d_%d", i, j)
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", callerNode, caller.Name))
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", callerNode, changeNode))
		}
	}

	sb.WriteString("```\n")
	return sb.String(), nil
}
