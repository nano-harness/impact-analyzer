package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// LLMContextOptions configures LLM-friendly output
type LLMContextOptions struct {
	MaxTokens         int     // Maximum tokens (0 = unlimited)
	TokenizerEstimate float64 // Characters per token estimate (default 4.0)
}

// LLMContextReporter generates LLM-friendly impact analysis reports
type LLMContextReporter struct {
	opts LLMContextOptions
}

// NewLLMContextReporter creates a new LLM context reporter
func NewLLMContextReporter(maxTokens int) *LLMContextReporter {
	return &LLMContextReporter{
		opts: LLMContextOptions{
			MaxTokens:         maxTokens,
			TokenizerEstimate: 4.0, // Approximate: 1 token ≈ 4 characters
		},
	}
}

// Generate generates an LLM-friendly report from impact results
func (r *LLMContextReporter) Generate(result *model.ImpactResult) (string, error) {
	if result == nil {
		return "No impact analysis results available.\n", nil
	}

	var sb strings.Builder

	// Header with summary
	sb.WriteString("# Code Change Impact Analysis\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Breaking Changes**: %d (require immediate attention)\n", result.BreakingCount))
	sb.WriteString(fmt.Sprintf("- **Warnings**: %d (potentially risky changes)\n", result.WarningCount))
	sb.WriteString(fmt.Sprintf("- **Safe Changes**: %d (low risk)\n", result.SafeCount))
	sb.WriteString(fmt.Sprintf("- **Total Reach**: %d symbols affected\n", result.TotalReach))
	sb.WriteString(fmt.Sprintf("- **Affected Tests**: %d\n\n", len(result.AffectedTests)))

	// Prioritize breaking changes
	breaking := r.filterByRisk(result.Changes, model.RiskLevelBreaking)
	warning := r.filterByRisk(result.Changes, model.RiskLevelWarning)
	safe := r.filterByRisk(result.Changes, model.RiskLevelSafe)

	// Sort each group by reach (most impactful first)
	r.sortByReach(breaking)
	r.sortByReach(warning)
	r.sortByReach(safe)

	// Breaking changes section (always include)
	if len(breaking) > 0 {
		sb.WriteString("## ⚠️  Breaking Changes\n\n")
		sb.WriteString("These changes break backward compatibility and require immediate attention:\n\n")
		for i, impact := range breaking {
			if r.shouldTruncate(&sb, result) {
				sb.WriteString(fmt.Sprintf("\n... and %d more breaking changes (truncated)\n", len(breaking)-i))
				break
			}
			r.writeChangeImpact(&sb, impact, i+1)
		}
		sb.WriteString("\n")
	}

	// Warning changes section (include if space allows)
	if len(warning) > 0 && !r.shouldTruncate(&sb, result) {
		sb.WriteString("## ⚡ Warnings\n\n")
		sb.WriteString("These changes may cause issues:\n\n")
		for i, impact := range warning {
			if r.shouldTruncate(&sb, result) {
				sb.WriteString(fmt.Sprintf("\n... and %d more warnings (truncated)\n", len(warning)-i))
				break
			}
			r.writeChangeImpact(&sb, impact, i+1)
		}
		sb.WriteString("\n")
	}

	// Safe changes (only summary if space limited)
	if len(safe) > 0 && !r.shouldTruncate(&sb, result) {
		sb.WriteString("## ✅ Safe Changes\n\n")
		sb.WriteString(fmt.Sprintf("%d safe changes with low risk. Details available in full report.\n\n", len(safe)))
	}

	// Affected tests summary
	if len(result.AffectedTests) > 0 && !r.shouldTruncate(&sb, result) {
		sb.WriteString("## 🧪 Tests to Run\n\n")
		testsByFile := r.groupTestsByFile(result.AffectedTests)
		for file, tests := range testsByFile {
			if r.shouldTruncate(&sb, result) {
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %d tests\n", file, len(tests)))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// writeChangeImpact writes a single change impact entry
func (r *LLMContextReporter) writeChangeImpact(sb *strings.Builder, impact *model.ChangeImpact, index int) {
	change := impact.Change
	if change == nil || change.Symbol == nil {
		return
	}
	sb.WriteString(fmt.Sprintf("%d. **%s** `%s` in `%s:%d`\n",
		index,
		change.Type,
		change.Symbol.QualifiedName(),
		change.File,
		change.StartLine))

	sb.WriteString(fmt.Sprintf("   - Risk Score: %.1f\n", impact.RiskScore))
	sb.WriteString(fmt.Sprintf("   - Reach: %d symbols\n", impact.Reach))

	if len(impact.DirectCallers) > 0 {
		callerNames := r.getSymbolNames(impact.DirectCallers, 5)
		sb.WriteString(fmt.Sprintf("   - Direct Callers: %s\n", strings.Join(callerNames, ", ")))
	}

	if change.SignatureChanged {
		sb.WriteString("   - ⚠️  Signature changed\n")
	}
	if change.OutOfFunction {
		sb.WriteString(fmt.Sprintf("   - Out-of-function fallback: %s\n", change.FallbackKind))
	}
	if change.Symbol.IsExported {
		sb.WriteString("   - 📦 Public API\n")
	}

	sb.WriteString("\n")
}

// filterByRisk filters changes by risk level
func (r *LLMContextReporter) filterByRisk(changes []*model.ChangeImpact, level model.RiskLevel) []*model.ChangeImpact {
	var result []*model.ChangeImpact
	for _, change := range changes {
		if change.RiskLevel == level {
			result = append(result, change)
		}
	}
	return result
}

// sortByReach sorts changes by reach (descending)
func (r *LLMContextReporter) sortByReach(changes []*model.ChangeImpact) {
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Reach > changes[j].Reach
	})
}

// getSymbolNames returns a limited list of symbol names
func (r *LLMContextReporter) getSymbolNames(symbols []*model.Symbol, limit int) []string {
	var names []string
	for i, sym := range symbols {
		if i >= limit {
			names = append(names, fmt.Sprintf("... and %d more", len(symbols)-limit))
			break
		}
		names = append(names, sym.QualifiedName())
	}
	return names
}

// groupTestsByFile groups tests by file path
func (r *LLMContextReporter) groupTestsByFile(tests []*model.Symbol) map[string][]*model.Symbol {
	result := make(map[string][]*model.Symbol)
	for _, test := range tests {
		result[test.File] = append(result[test.File], test)
	}
	return result
}

// shouldTruncate checks if we should stop adding content due to token limit
func (r *LLMContextReporter) shouldTruncate(sb *strings.Builder, result *model.ImpactResult) bool {
	if r.opts.MaxTokens <= 0 {
		return false // Unlimited
	}

	currentLength := sb.Len()
	estimatedTokens := float64(currentLength) / r.opts.TokenizerEstimate

	// Leave 10% buffer for final sections
	return estimatedTokens > float64(r.opts.MaxTokens)*0.9
}
