package orchestrator

// Precision/recall evaluation harness for impact analysis.
//
// Each evalScenario builds a throwaway git repository via testutil.NewMiniRepo,
// commits a baseline, applies a change commit, and runs AnalyzeDiffWithSurface.
// The scenario declares the symbols that MUST be reported as affected
// (ExpectedAffected), the symbols that MUST NOT (ExpectedUnaffected), test
// mapping expectations, and risk-level ceilings. The harness computes
// precision/recall over these labeled sets, logs them per scenario, and can
// write a JSON summary when IMPACT_EVAL_REPORT points at a file path.
//
// Assertions are hard by default: a recall miss or a false positive fails the
// test. When current behavior is known to be wrong, the scenario lists the
// assertion key in KnownFailures with a root-cause note; such failures are
// logged and reported instead of failing the test, so the assertion stays
// strict and starts failing again (as "resolved") once the bug is fixed.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

// evalMaxDepth is the BFS depth used for all scenarios; deep enough for the
// 2-hop chains used here, shallow enough to keep runs fast.
const evalMaxDepth = 3

// evalSymbolSpec matches a symbol by name with optional package/owner/file
// constraints. An empty field means "don't care". File is matched as a
// repo-relative slash-suffixed path (caller symbols carry absolute paths).
type evalSymbolSpec struct {
	Name    string // required
	Package string // optional, e.g. "lib" or "com.example.lib"
	Owner   string // optional enclosing type
	File    string // optional repo-relative path, e.g. "svc/svc.go"
}

func (s evalSymbolSpec) String() string {
	q := s.Name
	if s.Owner != "" {
		q = s.Owner + "." + q
	}
	if s.Package != "" {
		q = s.Package + "." + q
	}
	if s.File != "" {
		q += "@" + s.File
	}
	return q
}

func (s evalSymbolSpec) matches(sym *model.Symbol) bool {
	if sym == nil || sym.Name != s.Name {
		return false
	}
	if s.Package != "" && sym.Package != s.Package {
		return false
	}
	if s.Owner != "" && sym.Owner != s.Owner {
		return false
	}
	if s.File != "" && !strings.HasSuffix(filepath.ToSlash(sym.File), s.File) {
		return false
	}
	return true
}

// evalRiskCheck constrains the classified risk of every change whose symbol
// matches Change: the level must stay within [MinLevel, MaxLevel]; an empty
// bound means unbounded. Reason documents why the band is the semantically
// correct one.
type evalRiskCheck struct {
	Change   evalSymbolSpec
	MinLevel model.RiskLevel
	MaxLevel model.RiskLevel
	Reason   string
}

// evalScenario is one labeled evaluation case.
type evalScenario struct {
	Name     string // unique, used as subtest name
	Language string // go/java/typescript/python/...
	Notes    string // free-form context for the report

	InitialFiles map[string]string // baseline commit (path -> content)
	ChangedFiles map[string]string // change commit (path -> content)

	ExpectedAffected   []evalSymbolSpec // must all be recalled (hard assertion)
	ExpectedUnaffected []evalSymbolSpec // must have zero matches (hard assertion)
	ExpectedTests      []evalSymbolSpec // must appear in result.AffectedTests
	UnexpectedTests    []evalSymbolSpec // must not appear in result.AffectedTests
	RiskChecks         []evalRiskCheck  // risk-level ceilings per change

	// KnownFailures maps assertion keys (as printed in failure messages) to
	// root-cause notes. A failed assertion listed here is logged and reported
	// but does not fail the test; a passing one is reported as resolved.
	KnownFailures map[string]string
}

// evalScenarioReport is the per-scenario machine-readable outcome.
type evalScenarioReport struct {
	Name           string   `json:"name"`
	Language       string   `json:"language"`
	Precision      float64  `json:"precision"` // over labeled sets; see precision_note
	Recall         float64  `json:"recall"`
	TP             int      `json:"tp"`
	FP             int      `json:"fp"` // labeled false positives only
	FN             int      `json:"fn"`
	Missed         []string `json:"missed,omitempty"`          // expected-affected symbols not recalled
	FalsePositives []string `json:"false_positives,omitempty"` // expected-unaffected symbols that matched
	RiskViolations []string `json:"risk_violations,omitempty"`
	FailedUnknown  []string `json:"failed_assertions,omitempty"` // failures NOT in KnownFailures
	KnownFailed    []string `json:"known_failures,omitempty"`    // failures listed in KnownFailures
	KnownResolved  []string `json:"known_resolved,omitempty"`    // KnownFailures keys that now pass
	AffectedCount  int      `json:"affected_count"`              // size of the predicted affected set
	Notes          string   `json:"notes,omitempty"`
}

// evalReport is the full JSON summary written to IMPACT_EVAL_REPORT.
type evalReport struct {
	GeneratedAt   string                `json:"generated_at"`
	PrecisionNote string                `json:"precision_note"`
	Scenarios     []*evalScenarioReport `json:"scenarios"`
	Overall       evalOverallReport     `json:"overall"`
}

type evalOverallReport struct {
	Scenarios     int     `json:"scenarios"`
	TP            int     `json:"tp"`
	FP            int     `json:"fp"`
	FN            int     `json:"fn"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	FailedUnknown int     `json:"failed_assertions"`
	KnownFailed   int     `json:"known_failures"`
	KnownResolved int     `json:"known_resolved"`
}

func riskRank(level model.RiskLevel) int {
	switch level {
	case model.RiskLevelSafe:
		return 0
	case model.RiskLevelWarning:
		return 1
	case model.RiskLevelBreaking:
		return 2
	}
	return -1
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// affectedSet flattens the symbols an analysis result marks as affected:
// direct and transitive callers of every change, plus interface
// implementations attached to interface changes.
func affectedSet(result *model.ImpactResult) []*model.Symbol {
	var affected []*model.Symbol
	for _, ch := range result.Changes {
		affected = append(affected, ch.DirectCallers...)
		affected = append(affected, ch.AllCallers...)
		affected = append(affected, ch.InterfaceImpls...)
	}
	return affected
}

func anyMatch(spec evalSymbolSpec, syms []*model.Symbol) *model.Symbol {
	for _, sym := range syms {
		if spec.matches(sym) {
			return sym
		}
	}
	return nil
}

// runEvalScenario executes one scenario and returns its report. Unknown
// assertion failures call t.Errorf; known ones are logged only.
func runEvalScenario(t *testing.T, sc *evalScenario) *evalScenarioReport {
	t.Helper()

	repo := testutil.NewMiniRepo(t)
	for _, path := range sortedKeys(sc.InitialFiles) {
		repo.WriteFile(t, path, sc.InitialFiles[path])
	}
	repo.Commit(t, "initial")
	for _, path := range sortedKeys(sc.ChangedFiles) {
		repo.WriteFile(t, path, sc.ChangedFiles[path])
	}
	repo.Commit(t, "change")

	orch, err := NewOrchestrator(repo.Dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	result, err := orch.AnalyzeDiffWithSurface("HEAD~1", evalMaxDepth, false)
	if err != nil {
		t.Fatalf("%s: AnalyzeDiffWithSurface: %v", sc.Name, err)
	}

	rep := &evalScenarioReport{Name: sc.Name, Language: sc.Language, Notes: sc.Notes}
	failedKeys := make(map[string]bool)
	fail := func(key, detail string) {
		failedKeys[key] = true
		if reason, known := sc.KnownFailures[key]; known {
			rep.KnownFailed = append(rep.KnownFailed, fmt.Sprintf("%s: %s (root cause: %s)", key, detail, reason))
			t.Logf("[%s] KNOWN FAILURE %s: %s (root cause: %s)", sc.Name, key, detail, reason)
			return
		}
		rep.FailedUnknown = append(rep.FailedUnknown, fmt.Sprintf("%s: %s", key, detail))
		t.Errorf("[%s] %s: %s", sc.Name, key, detail)
	}

	affected := affectedSet(result)
	rep.AffectedCount = len(affected)
	for _, ch := range result.Changes {
		if ch.Change == nil || ch.Change.Symbol == nil {
			continue
		}
		t.Logf("[%s] change: %s type=%s sig=%v body=%v risk=%s score=%.1f callers=%d impls=%d",
			sc.Name, ch.Change.Symbol.QualifiedName(), ch.Change.Type,
			ch.Change.SignatureChanged, ch.Change.BodyChanged,
			ch.RiskLevel, ch.RiskScore, len(ch.AllCallers), len(ch.InterfaceImpls))
	}

	// Recall assertions: every expected-affected symbol must be predicted.
	for _, spec := range sc.ExpectedAffected {
		key := "recall:" + spec.String()
		if anyMatch(spec, affected) == nil {
			rep.FN++
			rep.Missed = append(rep.Missed, spec.String())
			fail(key, "expected-affected symbol not reported as affected")
		} else {
			rep.TP++
		}
	}

	// Precision assertions: no expected-unaffected symbol may be predicted.
	for _, spec := range sc.ExpectedUnaffected {
		key := "precision:" + spec.String()
		if hit := anyMatch(spec, affected); hit != nil {
			rep.FP++
			rep.FalsePositives = append(rep.FalsePositives,
				fmt.Sprintf("%s (matched %s@%s)", spec.String(), hit.QualifiedName(), hit.File))
			fail(key, "expected-unaffected symbol was reported as affected")
		}
	}

	// Test-mapping assertions.
	for _, spec := range sc.ExpectedTests {
		key := "test:" + spec.String()
		if anyMatch(spec, result.AffectedTests) == nil {
			fail(key, "expected test not listed in affected_tests")
		}
	}
	for _, spec := range sc.UnexpectedTests {
		key := "test-absent:" + spec.String()
		if hit := anyMatch(spec, result.AffectedTests); hit != nil {
			fail(key, fmt.Sprintf("test %s@%s unexpectedly listed in affected_tests", hit.QualifiedName(), hit.File))
		}
	}

	// Risk-level bands.
	for _, check := range sc.RiskChecks {
		key := "risk:" + check.Change.String()
		matched := false
		for _, ch := range result.Changes {
			if ch.Change == nil || !check.Change.matches(ch.Change.Symbol) {
				continue
			}
			matched = true
			var detail string
			if check.MaxLevel != "" && riskRank(ch.RiskLevel) > riskRank(check.MaxLevel) {
				detail = fmt.Sprintf("risk=%s exceeds max %s (%s)", ch.RiskLevel, check.MaxLevel, check.Reason)
			}
			if check.MinLevel != "" && riskRank(ch.RiskLevel) < riskRank(check.MinLevel) {
				detail = fmt.Sprintf("risk=%s below min %s (%s)", ch.RiskLevel, check.MinLevel, check.Reason)
			}
			if detail != "" {
				rep.RiskViolations = append(rep.RiskViolations, detail)
				fail(key, detail)
			}
		}
		if !matched {
			fail(key, "risk check matched no change")
		}
	}

	// A known-failure key that did not fail means the underlying bug is fixed.
	for key, reason := range sc.KnownFailures {
		if !failedKeys[key] {
			rep.KnownResolved = append(rep.KnownResolved, fmt.Sprintf("%s (was: %s)", key, reason))
			t.Logf("[%s] RESOLVED known failure %s (was: %s) — consider removing it from KnownFailures", sc.Name, key, reason)
		}
	}
	sort.Strings(rep.Missed)
	sort.Strings(rep.FalsePositives)
	sort.Strings(rep.KnownFailed)
	sort.Strings(rep.KnownResolved)

	rep.Precision, rep.Recall = 1, 1
	if rep.TP+rep.FP > 0 {
		rep.Precision = float64(rep.TP) / float64(rep.TP+rep.FP)
	}
	if rep.TP+rep.FN > 0 {
		rep.Recall = float64(rep.TP) / float64(rep.TP+rep.FN)
	}
	t.Logf("[%s] precision=%.2f recall=%.2f (tp=%d fp=%d fn=%d, affected=%d)",
		sc.Name, rep.Precision, rep.Recall, rep.TP, rep.FP, rep.FN, rep.AffectedCount)
	return rep
}

// TestEval_PrecisionRecall runs every registered evaluation scenario and
// reports per-scenario precision/recall. With IMPACT_EVAL_REPORT set, it also
// writes a JSON summary to that path.
func TestEval_PrecisionRecall(t *testing.T) {
	scenarios := evalScenarios()
	reports := make([]*evalScenarioReport, 0, len(scenarios))
	for i := range scenarios {
		sc := &scenarios[i]
		t.Run(sc.Name, func(t *testing.T) {
			reports = append(reports, runEvalScenario(t, sc))
		})
	}

	report := &evalReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		PrecisionNote: "precision/recall are computed over the labeled symbol sets only: " +
			"fp counts predicted symbols that match expectedUnaffected; unlabeled predicted " +
			"symbols are ignored, so reported precision is an upper bound",
		Scenarios: reports,
	}
	for _, rep := range reports {
		o := &report.Overall
		o.Scenarios++
		o.TP += rep.TP
		o.FP += rep.FP
		o.FN += rep.FN
		o.FailedUnknown += len(rep.FailedUnknown)
		o.KnownFailed += len(rep.KnownFailed)
		o.KnownResolved += len(rep.KnownResolved)
	}
	report.Overall.Precision, report.Overall.Recall = 1, 1
	if report.Overall.TP+report.Overall.FP > 0 {
		report.Overall.Precision = float64(report.Overall.TP) / float64(report.Overall.TP+report.Overall.FP)
	}
	if report.Overall.TP+report.Overall.FN > 0 {
		report.Overall.Recall = float64(report.Overall.TP) / float64(report.Overall.TP+report.Overall.FN)
	}
	t.Logf("overall: precision=%.2f recall=%.2f (tp=%d fp=%d fn=%d), known_failures=%d, unexpected_failures=%d",
		report.Overall.Precision, report.Overall.Recall,
		report.Overall.TP, report.Overall.FP, report.Overall.FN,
		report.Overall.KnownFailed, report.Overall.FailedUnknown)

	if path := os.Getenv("IMPACT_EVAL_REPORT"); path != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatalf("marshal eval report: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write eval report %s: %v", path, err)
		}
		t.Logf("eval report written to %s", path)
	}
}
