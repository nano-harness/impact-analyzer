package report

import "github.com/nano-harness/impact-analyzer/internal/model"

func fixedImpactResult() *model.ImpactResult {
	target := &model.Symbol{Name: "Target", Package: "pkg", File: "pkg/target.go", Line: 7, Language: model.LanguageGo, IsExported: true}
	caller := &model.Symbol{Name: "Caller", Package: "pkg", File: "pkg/caller.go", Line: 3, Language: model.LanguageGo}
	return &model.ImpactResult{
		SchemaVersion: "1.0",
		GeneratedAt:   "2026-05-05T00:00:00Z",
		Changes: []*model.ChangeImpact{{
			Change:        &model.Change{Type: model.ChangeTypeModified, Symbol: target, File: target.File, StartLine: target.Line, SignatureChanged: true},
			RiskLevel:     model.RiskLevelBreaking,
			RiskScore:     100,
			Reach:         1,
			DirectCallers: []*model.Symbol{caller},
			AllCallers:    []*model.Symbol{caller},
		}},
		TotalReach:    1,
		BreakingCount: 1,
		WarningCount:  0,
		SafeCount:     0,
	}
}
