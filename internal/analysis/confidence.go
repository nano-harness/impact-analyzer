package analysis

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ConfidenceScorer scores unreachable symbols without making hard assertions.
type ConfidenceScorer struct {
	cfg                config.OrphanConfidence
	changedFiles       map[string]bool
	highGateMinCallers int
}

const largeBodyMinLines = 10

func NewConfidenceScorer(cfg config.OrphanConfidence, changedFiles []string, highGateMinCallers ...int) *ConfidenceScorer {
	m := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		m[filepath.ToSlash(f)] = true
	}
	gate := 0
	if len(highGateMinCallers) > 0 {
		gate = highGateMinCallers[0]
	}
	return &ConfidenceScorer{cfg: cfg, changedFiles: m, highGateMinCallers: gate}
}

type NoiseFlags struct {
	IsLocalVar         bool
	IsUnexportedConst  bool
	IsTinyStruct       bool
	IsShortFunc        bool
	SSAConfirmedOrphan bool // SSA confirmed this symbol is unreachable
}

func (s *ConfidenceScorer) Score(sym *model.Symbol, cg *graph.CallGraph, onlyTestCallers bool, noiseFlags ...interface{}) *model.OrphanCandidate {
	score := 1.0
	reasons := []string{"not reachable from known entry points"}
	file := filepath.ToSlash(sym.File)
	if s.isChangedFile(file) {
		score += s.cfg.NewCodeWeight
		reasons = append(reasons, "symbol is in the analyzed diff")
	}
	if len(cg.GetCallers(sym)) == 0 && len(cg.GetCallees(sym)) == 0 {
		score += s.cfg.FileIsolatedWeight
		reasons = append(reasons, "symbol has no detected callers or callees")
	}
	switch sym.Kind {
	case model.SymbolKindFunction, model.SymbolKindMethod, model.SymbolKindInterface:
		score += s.cfg.KindFunctionWeight
	case model.SymbolKindVariable:
		score += s.cfg.KindVariableOffset
	case model.SymbolKindConstant:
		if !sym.IsExported {
			score += s.cfg.KindUnexportedConstOffset
		}
	}
	if !sym.IsExported {
		score += s.cfg.UnexportedOffset
		reasons = append(reasons, "unexported symbol lowers confidence")
	}
	if (sym.Kind == model.SymbolKindFunction || sym.Kind == model.SymbolKindMethod) && (sym.EndLine-sym.Line) >= largeBodyMinLines {
		score += s.cfg.LargeBodyWeight
	}
	if onlyTestCallers {
		score += s.cfg.OnlyTestCallersOffset
		reasons = append(reasons, "only referenced by tests")
	}
	if sym.Language == model.LanguagePython {
		score += s.cfg.PythonDynamicOffset
		reasons = append(reasons, "Python dynamic dispatch/imports make confidence conservative")
	}

	// Parse variadic args: NoiseFlags and/or ReachState
	var flags NoiseFlags
	var reachState ReachState
	for _, arg := range noiseFlags {
		switch v := arg.(type) {
		case NoiseFlags:
			flags = v
		case ReachState:
			reachState = v
		}
	}

	if flags != (NoiseFlags{}) {
		if flags.IsLocalVar {
			score += s.cfg.LocalVariablePenalty
			reasons = append(reasons, "local variable lowers confidence")
		}
		if flags.IsUnexportedConst {
			score += s.cfg.UnexportedConstPenalty
			reasons = append(reasons, "unexported constant lowers confidence")
		}
		if flags.IsTinyStruct {
			score += s.cfg.TinyStructPenalty
			reasons = append(reasons, "tiny struct lowers confidence")
		}
		if flags.IsShortFunc {
			score += s.cfg.ShortFunctionPenalty
			reasons = append(reasons, "short function lowers confidence")
		}
		if flags.SSAConfirmedOrphan {
			score += s.cfg.SSAConfirmedWeight
			reasons = append(reasons, "SSA/RTA confirms symbol is unreachable")
		}
	}

	// Apply weak reach offset
	if reachState == WeakReach {
		score += s.cfg.WeakReachOffset
		reasons = append(reasons, "only reachable via type reference (weak reach)")
	}

	level := levelForScore(score)
	callerCount := len(cg.GetCallers(sym))
	if level == model.ConfidenceHigh && s.highGateMinCallers > 0 && callerCount >= s.highGateMinCallers {
		level = model.ConfidenceMedium
		reasons = append(reasons, fmt.Sprintf("has %d+ callers, demoted from high", callerCount))
	}
	return &model.OrphanCandidate{Symbol: sym, Confidence: level, Score: score, Reasons: reasons, Suggestions: suggestions(sym)}
}

func (s *ConfidenceScorer) isChangedFile(file string) bool {
	if s.changedFiles[file] {
		return true
	}
	for changed := range s.changedFiles {
		if changed != "" && strings.HasSuffix(file, "/"+changed) {
			return true
		}
	}
	return false
}

func levelForScore(score float64) model.ConfidenceLevel {
	switch {
	case score >= 4:
		return model.ConfidenceHigh
	case score >= 2:
		return model.ConfidenceMedium
	default:
		return model.ConfidenceLow
	}
}

func suggestions(sym *model.Symbol) []string {
	name := sym.Name
	if q := sym.QualifiedName(); q != "" {
		name = q
	}
	return []string{
		fmt.Sprintf("Verify whether %s should be wired into a framework route, scheduler, CLI, or explicit extra entry", name),
		"If intentional dead code, remove it or add the path to orphan.ignore",
		"If this is a dynamic entry point, add it to orphan.extra_entries",
	}
}

func confidenceFromString(value string) model.ConfidenceLevel {
	switch strings.ToLower(value) {
	case "high":
		return model.ConfidenceHigh
	case "low":
		return model.ConfidenceLow
	default:
		return model.ConfidenceMedium
	}
}
