package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// SurfaceReporter generates reports for contract surface changes
type SurfaceReporter interface {
	GenerateSurface(diff *model.ContractDiff) (string, error)
}

// SurfaceTextReporter generates text reports for surface changes
type SurfaceTextReporter struct{}

// GenerateSurface generates a text report for contract diff
func (r *SurfaceTextReporter) GenerateSurface(diff *model.ContractDiff) (string, error) {
	if diff == nil || diff.IsEmpty() {
		return "No contract changes detected.\n", nil
	}

	var sb strings.Builder

	// Header
	sb.WriteString(color.New(color.Bold).Sprint("🔌 Public Contract Changes"))
	sb.WriteString(fmt.Sprintf(" (%s)\n\n", diff.Summary()))

	// Added contracts
	if len(diff.ContractsAdded) > 0 {
		sb.WriteString(color.GreenString("✅ Added (%d):\n", len(diff.ContractsAdded)))
		for _, contract := range diff.ContractsAdded {
			sb.WriteString(fmt.Sprintf("  + [%s] %s\n", contract.Kind, contract.Identifier))
			sb.WriteString(fmt.Sprintf("    File: %s:%d\n", contract.File, contract.Line))
			if contract.Framework != "" {
				sb.WriteString(fmt.Sprintf("    Framework: %s\n", contract.Framework))
			}
			writeSurfaceHandler(&sb, contract, "    ")
		}
		sb.WriteString("\n")
	}

	// Removed contracts
	if len(diff.ContractsRemoved) > 0 {
		sb.WriteString(color.RedString("❌ Removed (%d):\n", len(diff.ContractsRemoved)))
		for _, contract := range diff.ContractsRemoved {
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", contract.Kind, contract.Identifier))
			sb.WriteString(fmt.Sprintf("    File: %s:%d\n", contract.File, contract.Line))
			if contract.Framework != "" {
				sb.WriteString(fmt.Sprintf("    Framework: %s\n", contract.Framework))
			}
			writeSurfaceHandler(&sb, contract, "    ")
		}
		sb.WriteString("\n")
	}

	// Modified contracts
	if len(diff.ContractsModified) > 0 {
		sb.WriteString(color.YellowString("⚠️  Modified (%d):\n", len(diff.ContractsModified)))
		for _, contract := range diff.ContractsModified {
			sb.WriteString(fmt.Sprintf("  ~ [%s] %s\n", contract.Kind, contract.Identifier))
			sb.WriteString(fmt.Sprintf("    File: %s:%d\n", contract.File, contract.Line))
			if contract.Framework != "" {
				sb.WriteString(fmt.Sprintf("    Framework: %s\n", contract.Framework))
			}
			writeSurfaceHandler(&sb, contract, "    ")
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// SurfaceJSONReporter generates JSON reports for surface changes
type SurfaceJSONReporter struct{}

// GenerateSurface generates a JSON report for contract diff
func (r *SurfaceJSONReporter) GenerateSurface(diff *model.ContractDiff) (string, error) {
	if diff == nil {
		diff = &model.ContractDiff{
			ContractsAdded:    []*model.Contract{},
			ContractsRemoved:  []*model.Contract{},
			ContractsModified: []*model.Contract{},
		}
	}

	// Create a structured output
	output := map[string]interface{}{
		"schema_version": "1.0",
		"summary": map[string]int{
			"added":    len(diff.ContractsAdded),
			"removed":  len(diff.ContractsRemoved),
			"modified": len(diff.ContractsModified),
		},
		"added":    diff.ContractsAdded,
		"removed":  diff.ContractsRemoved,
		"modified": diff.ContractsModified,
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// SurfaceLLMContextReporter generates LLM-friendly reports for surface changes
type SurfaceLLMContextReporter struct {
	MaxTokens int
}

// GenerateSurface generates an LLM-friendly report for contract diff
func (r *SurfaceLLMContextReporter) GenerateSurface(diff *model.ContractDiff) (string, error) {
	if diff == nil || diff.IsEmpty() {
		return "No public contract changes detected in this diff.\n", nil
	}

	var sb strings.Builder

	// Summary
	sb.WriteString("## Public Contract Changes Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Added**: %d new contracts\n", len(diff.ContractsAdded)))
	sb.WriteString(fmt.Sprintf("- **Removed**: %d contracts (⚠️ BREAKING)\n", len(diff.ContractsRemoved)))
	sb.WriteString(fmt.Sprintf("- **Modified**: %d contracts (⚠️ potentially breaking)\n\n", len(diff.ContractsModified)))

	// Breaking changes (removed)
	if len(diff.ContractsRemoved) > 0 {
		sb.WriteString("### ❌ Removed Contracts (Breaking Changes)\n\n")
		for _, contract := range diff.ContractsRemoved {
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", contract.Kind, contract.Identifier))
			sb.WriteString(fmt.Sprintf("  - Location: `%s:%d`\n", contract.File, contract.Line))
			if contract.Framework != "" {
				sb.WriteString(fmt.Sprintf("  - Framework: %s\n", contract.Framework))
			}
			writeSurfaceHandler(&sb, contract, "  - ")
		}
		sb.WriteString("\n")
	}

	// Modified contracts
	if len(diff.ContractsModified) > 0 {
		sb.WriteString("### ⚠️  Modified Contracts\n\n")
		for _, contract := range diff.ContractsModified {
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", contract.Kind, contract.Identifier))
			sb.WriteString(fmt.Sprintf("  - Location: `%s:%d`\n", contract.File, contract.Line))
			if contract.Framework != "" {
				sb.WriteString(fmt.Sprintf("  - Framework: %s\n", contract.Framework))
			}
			writeSurfaceHandler(&sb, contract, "  - ")
		}
		sb.WriteString("\n")
	}

	// New contracts
	if len(diff.ContractsAdded) > 0 {
		sb.WriteString("### ✅ New Contracts\n\n")
		for _, contract := range diff.ContractsAdded {
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", contract.Kind, contract.Identifier))
			sb.WriteString(fmt.Sprintf("  - Location: `%s:%d`\n", contract.File, contract.Line))
			if contract.Framework != "" {
				sb.WriteString(fmt.Sprintf("  - Framework: %s\n", contract.Framework))
			}
			writeSurfaceHandler(&sb, contract, "  - ")
		}
		sb.WriteString("\n")
	}

	result := sb.String()

	// Token limiting (approximate)
	if r.MaxTokens > 0 {
		// Rough estimate: 1 token ≈ 4 characters
		maxChars := r.MaxTokens * 4
		if len(result) > maxChars {
			result = result[:maxChars] + "\n\n... (output truncated to fit token limit)"
		}
	}

	return result, nil
}

func writeSurfaceHandler(sb *strings.Builder, contract *model.Contract, prefix string) {
	if contract == nil || contract.Signature == nil {
		return
	}
	handler, ok := contract.Signature["handler"].(string)
	if !ok || handler == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("%sHandler: %s\n", prefix, handler))
}
