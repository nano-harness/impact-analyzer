package cmd

import (
	"encoding/json"
	"fmt"
)

// GraphJSONOutput represents the JSON output structure for graph command
type GraphJSONOutput struct {
	SchemaVersion string   `json:"schema_version"`
	Target        string   `json:"target"`
	Reverse       bool     `json:"reverse"`
	Entries       []string `json:"entries,omitempty"`      // reverse mode: cmd entry files list
	Dependencies  []string `json:"dependencies,omitempty"` // forward mode: files this target depends on
	Total         int      `json:"total"`
}

// renderDependencyJSON renders dependency graph as JSON
func renderDependencyJSON(targetFile string, depMap map[string][]string, reverse bool) (string, error) {
	output := GraphJSONOutput{
		SchemaVersion: "1.0",
		Target:        targetFile,
		Reverse:       reverse,
	}

	deps := depMap[targetFile]
	output.Total = len(deps)

	if reverse {
		// In reverse mode, these are files that depend on the target (entries)
		output.Entries = deps
	} else {
		// In forward mode, these are files that the target depends on
		output.Dependencies = deps
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}
