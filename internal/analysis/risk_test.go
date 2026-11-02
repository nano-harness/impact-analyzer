package analysis

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestRiskClassifierHardRulesAndRanges(t *testing.T) {
	cases := []struct {
		name   string
		impact *model.ChangeImpact
		want   model.RiskLevel
		score  float64
	}{
		{"added unreachable is safe", riskImpact(false, 0, false, model.ChangeTypeAdded), model.RiskLevelSafe, 0},
		{"deleted exported is breaking", riskImpact(true, 0, false, model.ChangeTypeDeleted), model.RiskLevelBreaking, 100},
		{"exported signature with caller is breaking", riskImpact(true, 1, true, model.ChangeTypeModified), model.RiskLevelBreaking, 100},
		{"private many callers warns", riskImpact(false, 10, false, model.ChangeTypeModified), model.RiskLevelWarning, 50},
	}

	rc := NewRiskClassifier()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, score := rc.ClassifyRisk(tc.impact)
			if level != tc.want {
				t.Fatalf("level=%s, want %s", level, tc.want)
			}
			if score != tc.score {
				t.Fatalf("score=%v, want %v", score, tc.score)
			}
		})
	}
}

func TestRiskClassifierThresholdBoundaries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Risk.BreakingThreshold = 45
	cfg.Risk.WarningThreshold = 35
	rc := NewRiskClassifierFromConfig(cfg)

	level, score := rc.ClassifyRisk(riskImpact(false, 9, false, model.ChangeTypeModified))
	if level != model.RiskLevelBreaking || score != 45 {
		t.Fatalf("score at breaking threshold got (%s, %.1f), want breaking 45.0", level, score)
	}

	level, score = rc.ClassifyRisk(riskImpact(true, 1, false, model.ChangeTypeModified))
	if level != model.RiskLevelWarning || score != 35 {
		t.Fatalf("score at warning threshold got (%s, %.1f), want warning 35.0", level, score)
	}
}

func TestRiskClassifierPublicAPINotLowerThanPrivateHotspot(t *testing.T) {
	rc := NewRiskClassifier()
	_, privateScore := rc.ClassifyRisk(riskImpact(false, 100, false, model.ChangeTypeModified))
	_, publicScore := rc.ClassifyRisk(riskImpact(true, 1, true, model.ChangeTypeModified))
	if publicScore < privateScore {
		t.Fatalf("public API score %.1f should not be lower than private hotspot %.1f", publicScore, privateScore)
	}
}

func riskImpact(exported bool, reach int, sigChanged bool, changeType model.ChangeType) *model.ChangeImpact {
	return &model.ChangeImpact{
		Change: &model.Change{
			Type:             changeType,
			SignatureChanged: sigChanged,
			Symbol:           &model.Symbol{Name: "Target", IsExported: exported},
		},
		Reach: reach,
	}
}
