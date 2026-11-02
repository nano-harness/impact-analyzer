package model

import "fmt"

// RiskLevel represents the risk level of a change
type RiskLevel string

const (
	RiskLevelBreaking RiskLevel = "breaking"
	RiskLevelWarning  RiskLevel = "warning"
	RiskLevelSafe     RiskLevel = "safe"
)

// Color returns the terminal color for the risk level
func (r RiskLevel) Color() string {
	switch r {
	case RiskLevelBreaking:
		return "red"
	case RiskLevelWarning:
		return "yellow"
	case RiskLevelSafe:
		return "green"
	default:
		return "white"
	}
}

// AffectedContract represents a contract affected by a method change
type AffectedContract struct {
	Contract  *Contract `json:"contract"`
	CallPath  []*Symbol `json:"call_path"`  // From changed method -> handler
	Depth     int       `json:"depth"`      // Length of call path
	RiskLevel RiskLevel `json:"risk_level"` // Risk level of the change
}

// ChangeImpact represents the impact of a single change
type ChangeImpact struct {
	Change         *Change   `json:"change"`
	RiskLevel      RiskLevel `json:"risk_level"`
	RiskScore      float64   `json:"risk_score"` // 0-100
	DirectCallers  []*Symbol `json:"direct_callers"`
	AllCallers     []*Symbol `json:"all_callers"` // Full transitive closure
	Reach          int       `json:"reach"`       // Total number of affected symbols
	AffectedTests  []*Symbol `json:"affected_tests"`
	InterfaceImpls []*Symbol `json:"interface_impls,omitempty"` // For interface changes

	// Detailed metrics
	ExportedCallersCount int `json:"exported_callers_count"`
	InternalCallersCount int `json:"internal_callers_count"`
	CrossPackageCallers  int `json:"cross_package_callers"`

	// Affected contracts (forward trace: method -> contracts)
	AffectedContracts []*AffectedContract `json:"affected_contracts,omitempty"`
}

// ImpactResult represents the complete impact analysis result
type ImpactResult struct {
	SchemaVersion string `json:"schema_version"` // Schema version for compatibility
	GeneratedAt   string `json:"generated_at"`   // Timestamp of generation

	Changes       []*ChangeImpact `json:"changes"`
	TotalReach    int             `json:"total_reach"`
	BreakingCount int             `json:"breaking_count"`
	WarningCount  int             `json:"warning_count"`
	SafeCount     int             `json:"safe_count"`

	// Aggregate metrics
	TotalCallers     int       `json:"total_callers"`
	AffectedFiles    []string  `json:"affected_files"`
	AffectedPackages []string  `json:"affected_packages"`
	AffectedTests    []*Symbol `json:"affected_tests"`

	// Language breakdown
	LanguageBreakdown map[Language]int `json:"language_breakdown"`

	// Surface changes (REQ-002: integrate contract diff)
	SurfaceDiff *ContractDiff `json:"surface_diff,omitempty"`

	// Aggregate contract impact metrics
	AffectedContractsCount int `json:"affected_contracts_count,omitempty"`
}

// Summary returns a human-readable summary
func (ir *ImpactResult) Summary() string {
	return fmt.Sprintf("Breaking: %d, Warnings: %d, Safe: %d, Total reach: %d symbols",
		ir.BreakingCount, ir.WarningCount, ir.SafeCount, ir.TotalReach)
}
