package analysis

import (
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// EntryPointChecker is an interface for checking if a symbol is an entry point
// Entry points are boundaries where call graph traversal should stop
type EntryPointChecker interface {
	IsEntryPoint(sym *model.Symbol) bool
}

// ImpactAnalyzer performs impact analysis on code changes
type ImpactAnalyzer struct {
	callGraph    *graph.CallGraph
	entryChecker EntryPointChecker // Optional entry point checker for stopping traversal
}

// NewImpactAnalyzer creates a new impact analyzer
func NewImpactAnalyzer(cg *graph.CallGraph) *ImpactAnalyzer {
	return &ImpactAnalyzer{callGraph: cg}
}

// NewImpactAnalyzerWithEntryChecker creates a new impact analyzer with an entry point checker
func NewImpactAnalyzerWithEntryChecker(cg *graph.CallGraph, ec EntryPointChecker) *ImpactAnalyzer {
	return &ImpactAnalyzer{
		callGraph:    cg,
		entryChecker: ec,
	}
}

// Analyze analyzes the impact of a set of changes
func (ia *ImpactAnalyzer) Analyze(changes []*model.Change, maxDepth int) (*model.ImpactResult, error) {
	result := &model.ImpactResult{
		Changes:           make([]*model.ChangeImpact, 0, len(changes)),
		AffectedFiles:     make([]string, 0, len(changes)),
		AffectedPackages:  make([]string, 0, len(changes)),
		AffectedTests:     make([]*model.Symbol, 0, len(changes)),
		LanguageBreakdown: make(map[model.Language]int),
	}

	for _, change := range changes {
		if change.Symbol == nil || change.NoiseFiltered {
			result.Changes = append(result.Changes, &model.ChangeImpact{Change: change})
			continue
		}

		// Build stop function for entry point checking
		var stopFn func(*model.Symbol) bool
		if ia.entryChecker != nil {
			stopFn = func(sym *model.Symbol) bool {
				return ia.entryChecker.IsEntryPoint(sym)
			}
		}

		impact := &model.ChangeImpact{
			Change:        change,
			DirectCallers: ia.callGraph.GetCallers(change.Symbol),
			AllCallers:    ia.callGraph.GetAllCallersWithStop(change.Symbol, maxDepth, stopFn),
		}

		impact.Reach = len(impact.AllCallers)

		// Note: Risk classification is now handled by RiskClassifier in orchestrator
		// We just collect the call graph data here

		result.Changes = append(result.Changes, impact)
		result.TotalReach += impact.Reach
	}

	return result, nil
}
