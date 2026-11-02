package analysis

import (
	"math"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// Default scoring weights. Config weights scale the corresponding score
// component relative to these values, so defaults reproduce the historical
// scoring exactly (zero config values fall back to these).
const (
	defaultCallerWeight    = 0.4
	defaultExportWeight    = 0.3
	defaultInterfaceWeight = 0.3
)

// RiskClassifier classifies changes by risk level
type RiskClassifier struct {
	breakingThreshold float64
	warningThreshold  float64
	callerWeight      float64
	exportWeight      float64
	interfaceWeight   float64
}

// NewRiskClassifier creates a new risk classifier with default values
func NewRiskClassifier() *RiskClassifier {
	return &RiskClassifier{
		breakingThreshold: 75.0,
		warningThreshold:  30.0,
		callerWeight:      defaultCallerWeight,
		exportWeight:      defaultExportWeight,
		interfaceWeight:   defaultInterfaceWeight,
	}
}

// NewRiskClassifierFromConfig creates a new risk classifier from config
func NewRiskClassifierFromConfig(cfg *config.Config) *RiskClassifier {
	rc := &RiskClassifier{
		breakingThreshold: 75.0,
		warningThreshold:  30.0,
		callerWeight:      defaultCallerWeight,
		exportWeight:      defaultExportWeight,
		interfaceWeight:   defaultInterfaceWeight,
	}

	// Override with config values if provided
	if cfg != nil {
		if cfg.Risk.BreakingThreshold > 0 {
			rc.breakingThreshold = cfg.Risk.BreakingThreshold
		}
		if cfg.Risk.WarningThreshold > 0 {
			rc.warningThreshold = cfg.Risk.WarningThreshold
		}
		if cfg.Risk.CallerWeight > 0 {
			rc.callerWeight = cfg.Risk.CallerWeight
		}
		if cfg.Risk.ExportWeight > 0 {
			rc.exportWeight = cfg.Risk.ExportWeight
		}
		if cfg.Risk.InterfaceWeight > 0 {
			rc.interfaceWeight = cfg.Risk.InterfaceWeight
		}
	}

	return rc
}

// ClassifyRisk calculates risk score and level for a change impact
func (rc *RiskClassifier) ClassifyRisk(impact *model.ChangeImpact) (model.RiskLevel, float64) {
	// Apply hard rules first (override scoring)
	if level, score, isHardRule := rc.applyHardRules(impact); isHardRule {
		return level, score
	}

	// Calculate score for non-hard-rule cases
	score := rc.calculateScore(impact)

	if score >= rc.breakingThreshold {
		return model.RiskLevelBreaking, score
	} else if score >= rc.warningThreshold {
		return model.RiskLevelWarning, score
	}
	return model.RiskLevelSafe, score
}

// applyHardRules applies hard-coded rules that override scoring
// Returns (level, score, isHardRule) where isHardRule indicates if a hard rule matched
func (rc *RiskClassifier) applyHardRules(impact *model.ChangeImpact) (model.RiskLevel, float64, bool) {
	isExported := impact.Change.Symbol != nil && impact.Change.Symbol.IsExported
	isInterface := len(impact.InterfaceImpls) > 0
	isDelete := impact.Change.Type == model.ChangeTypeDeleted
	isAdd := impact.Change.Type == model.ChangeTypeAdded
	sigChanged := impact.Change.SignatureChanged

	// Rule 1: Deleting an exported symbol is always breaking
	if isDelete && isExported {
		return model.RiskLevelBreaking, 100.0, true
	}

	// Rule 2: Interface changes (delete or signature change) are always breaking
	if isInterface && (isDelete || sigChanged) {
		return model.RiskLevelBreaking, 100.0, true
	}

	// Rule 3: Adding new code with no callers is always safe
	if isAdd && impact.Reach == 0 {
		return model.RiskLevelSafe, 0.0, true
	}

	// Rule 4: Exported signature changes with callers are breaking
	if isExported && sigChanged && impact.Reach > 0 {
		return model.RiskLevelBreaking, 100.0, true
	}

	return model.RiskLevelSafe, 0.0, false
}

func (rc *RiskClassifier) calculateScore(impact *model.ChangeImpact) float64 {
	// Caller score: normalize by reach, scaled by configured caller weight
	callerScore := math.Min(float64(impact.Reach)/10.0, 1.0) * 100 * (rc.callerWeight / defaultCallerWeight)

	// Export bonus: only applies when there are actual callers
	exportBonus := 0.0
	if impact.Change.Symbol != nil && impact.Change.Symbol.IsExported && impact.Reach > 0 {
		exportBonus = 30.0 * (rc.exportWeight / defaultExportWeight)
	}

	// Signature change bonus: strong signal of potential breakage
	sigBonus := 0.0
	if impact.Change.SignatureChanged {
		sigBonus = 40.0
	}

	// Interface score: higher if it affects interface implementations
	interfaceScore := 0.0
	if len(impact.InterfaceImpls) > 0 {
		interfaceScore = 30.0 * (rc.interfaceWeight / defaultInterfaceWeight)
	}

	// Composite score: reach is now weighted higher (50%)
	// Export/signature/interface are additional risk factors
	score := callerScore*0.5 + exportBonus + sigBonus + interfaceScore

	return score
}
