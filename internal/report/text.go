package report

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// TextReporter generates colored terminal text reports
type TextReporter struct{}

// NewTextReporter creates a new text reporter
func NewTextReporter() *TextReporter {
	return &TextReporter{}
}

// Generate generates a colored text report
func (tr *TextReporter) Generate(result *model.ImpactResult) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString(color.New(color.Bold).Sprint("Code Change Impact Analysis\n"))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Summary
	sb.WriteString(color.New(color.Bold).Sprint("Summary:\n"))
	sb.WriteString(fmt.Sprintf("  Total Changes: %d\n", len(result.Changes)))
	sb.WriteString(fmt.Sprintf("  %s Breaking: %d\n",
		color.New(color.FgRed).Sprint("●"),
		result.BreakingCount))
	sb.WriteString(fmt.Sprintf("  %s Warning: %d\n",
		color.New(color.FgYellow).Sprint("●"),
		result.WarningCount))
	sb.WriteString(fmt.Sprintf("  %s Safe: %d\n",
		color.New(color.FgGreen).Sprint("●"),
		result.SafeCount))
	sb.WriteString(fmt.Sprintf("  Total Reach: %d symbols\n\n", result.TotalReach))

	// Breaking Changes
	if result.BreakingCount > 0 {
		sb.WriteString(color.New(color.FgRed, color.Bold).Sprint("Breaking Changes:\n"))
		for _, change := range result.Changes {
			if change.RiskLevel == model.RiskLevelBreaking {
				tr.formatChange(&sb, change)
			}
		}
		sb.WriteString("\n")
	}

	// Warnings
	if result.WarningCount > 0 {
		sb.WriteString(color.New(color.FgYellow, color.Bold).Sprint("Warnings:\n"))
		for _, change := range result.Changes {
			if change.RiskLevel == model.RiskLevelWarning {
				tr.formatChange(&sb, change)
			}
		}
		sb.WriteString("\n")
	}

	// Safe Changes
	if result.SafeCount > 0 {
		sb.WriteString(color.New(color.FgGreen, color.Bold).Sprint("Safe Changes:\n"))
		for _, change := range result.Changes {
			if change.RiskLevel == model.RiskLevelSafe {
				tr.formatChange(&sb, change)
			}
		}
		sb.WriteString("\n")
	}

	// Affected Tests
	if len(result.AffectedTests) > 0 {
		sb.WriteString(color.New(color.Bold).Sprint("Affected Tests:\n"))
		for _, test := range result.AffectedTests {
			sb.WriteString(fmt.Sprintf("  - %s (%s:%d)\n",
				test.Name, test.File, test.Line))
		}
	}

	return sb.String(), nil
}

func (tr *TextReporter) formatChange(sb *strings.Builder, change *model.ChangeImpact) {
	if change.Change.Symbol == nil {
		return
	}

	symbol := change.Change.Symbol
	sb.WriteString(fmt.Sprintf("  %s %s.%s\n",
		tr.getRiskIndicator(change.RiskLevel),
		symbol.Package,
		symbol.Name))
	sb.WriteString(fmt.Sprintf("    File: %s:%d\n", symbol.File, symbol.Line))
	sb.WriteString(fmt.Sprintf("    Type: %s\n", change.Change.Type))
	sb.WriteString(fmt.Sprintf("    Reach: %d callers\n", change.Reach))

	if len(change.DirectCallers) > 0 && len(change.DirectCallers) <= 5 {
		sb.WriteString("    Direct Callers:\n")
		for _, caller := range change.DirectCallers {
			sb.WriteString(fmt.Sprintf("      - %s\n", caller.QualifiedName()))
		}
	} else if len(change.DirectCallers) > 5 {
		sb.WriteString(fmt.Sprintf("    Direct Callers: %d (too many to list)\n",
			len(change.DirectCallers)))
	}
	sb.WriteString("\n")
}

func (tr *TextReporter) getRiskIndicator(level model.RiskLevel) string {
	switch level {
	case model.RiskLevelBreaking:
		return color.New(color.FgRed).Sprint("✗")
	case model.RiskLevelWarning:
		return color.New(color.FgYellow).Sprint("⚠")
	case model.RiskLevelSafe:
		return color.New(color.FgGreen).Sprint("✓")
	default:
		return "?"
	}
}
