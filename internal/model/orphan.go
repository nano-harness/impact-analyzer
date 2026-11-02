package model

// ConfidenceLevel represents orphan detection confidence.
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"
	ConfidenceMedium ConfidenceLevel = "medium"
	ConfidenceLow    ConfidenceLevel = "low"
)

// OrphanCandidate is a symbol that appears unreachable from known entry points.
type OrphanCandidate struct {
	Symbol      *Symbol         `json:"symbol"`
	Confidence  ConfidenceLevel `json:"confidence"`
	Score       float64         `json:"score"`
	Reasons     []string        `json:"reasons"`
	Suggestions []string        `json:"suggestions,omitempty"`
}

// OrphanCluster groups related orphan candidates by package and file prefix.
type OrphanCluster struct {
	Package    string          `json:"package"`
	FilePrefix string          `json:"file_prefix"`
	Count      int             `json:"count"`
	Symbols    []string        `json:"symbols"`    // QualifiedNames
	Confidence ConfidenceLevel `json:"confidence"` // Max confidence in cluster
}

// OrphanDiagnostic represents a diagnostic message about analysis capabilities.
type OrphanDiagnostic struct {
	Level   string `json:"level"` // "info" | "warning" | "error"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"` // optional detail (e.g. error string)
}

// OrphanResult contains an orphan-code detection report.
type OrphanResult struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	Candidates    []*OrphanCandidate `json:"candidates"`
	EntryPoints   []*Symbol          `json:"entry_points,omitempty"`
	Clusters      []*OrphanCluster   `json:"clusters,omitempty"`
	Summary       OrphanSummary      `json:"summary"`
	Diagnostics   []OrphanDiagnostic `json:"diagnostics,omitempty"`
}

// OrphanSummary contains aggregate counts.
type OrphanSummary struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

// Rank returns an ordering value for confidence levels.
func (c ConfidenceLevel) Rank() int {
	switch c {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}
