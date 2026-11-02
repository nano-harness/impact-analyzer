package orchestrator

// End-to-end and unit coverage for cross-package caller recall: a caller in
// another package must be found when its file imports the changed package,
// and must not be attributed when it does not.

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

// E2E: other imports lib and calls lib.Helper(); third defines its own Helper
// and calls it without importing lib. Changing lib.Helper must surface
// other.Service as a caller and must NOT attribute third.UseHelper to it.
func TestE2E_CrossPackageRecall(t *testing.T) {
	repo := testutil.NewMiniRepo(t)

	repo.WriteFile(t, "go.mod", "module example.com/scratch\n\ngo 1.25\n")
	repo.WriteFile(t, "lib/lib.go", `package lib

func Helper() string {
	return "hello"
}
`)
	repo.WriteFile(t, "other/other.go", `package other

import "example.com/scratch/lib"

func Service() string {
	return lib.Helper()
}
`)
	repo.WriteFile(t, "third/third.go", `package third

func Helper() string {
	return "third"
}

func UseHelper() string {
	return Helper()
}
`)
	repo.Commit(t, "initial")

	repo.WriteFile(t, "lib/lib.go", `package lib

func Helper() string {
	return "world"
}
`)
	repo.Commit(t, "modify helper")

	orch, err := NewOrchestrator(repo.Dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	result, err := orch.AnalyzeDiffWithSurface("HEAD~1", 3, false)
	if err != nil {
		t.Fatal(err)
	}

	foundOther := false
	foundThird := false
	for _, change := range result.Changes {
		if change.Change == nil || change.Change.Symbol == nil || change.Change.Symbol.Name != "Helper" {
			continue
		}
		if change.Change.Symbol.Package != "lib" {
			continue
		}
		for _, caller := range change.AllCallers {
			t.Logf("caller of lib.Helper: %s.%s (file=%s)", caller.Package, caller.Name, caller.File)
			if caller.Package == "other" && caller.Name == "Service" {
				foundOther = true
			}
			if caller.Package == "third" {
				foundThird = true
			}
		}
	}
	if !foundOther {
		t.Errorf("RECALL MISS: expected other.Service as caller of lib.Helper")
	}
	if foundThird {
		t.Errorf("FALSE POSITIVE: third package caller attributed to lib.Helper")
	}
}

// Unit-level: single-candidate package guess is now validated against the
// calling file's context (same package or matching import).
func TestEnrich_SingleCandidateGuard(t *testing.T) {
	o := &Orchestrator{callGraph: graph.NewCallGraph()}
	o.callGraph.AddSymbol(&model.Symbol{Name: "Helper", Package: "lib"})

	// No import, different package: pre-fix guessed "lib", must now be "".
	if got := o.inferCalleePackage("Helper", &model.FileSymbols{Package: "lonely"}); got != "" {
		t.Errorf("no-context file: expected \"\", got %q", got)
	}
	// Importing file: still resolved to "lib" (Go full-path suffix match).
	if got := o.inferCalleePackage("Helper", &model.FileSymbols{
		Package: "other", Imports: []string{"example.com/scratch/lib"},
	}); got != "lib" {
		t.Errorf("importing file: expected \"lib\", got %q", got)
	}
	// Same package: resolved.
	if got := o.inferCalleePackage("Helper", &model.FileSymbols{Package: "lib"}); got != "lib" {
		t.Errorf("same package: expected \"lib\", got %q", got)
	}
	// Java-style class import under the package.
	if got := o.inferCalleePackage("Helper", &model.FileSymbols{
		Package: "other", Imports: []string{"lib.Helper"},
	}); got != "lib" {
		t.Errorf("java-style import: expected \"lib\", got %q", got)
	}
}

// Unit-level: bare-name cross-package caller kept only when the caller's file
// imports the target package; nil callback preserves the old drop behavior.
func TestGraph_ImportCallback(t *testing.T) {
	target := &model.Symbol{Name: "Helper", Package: "lib", File: "/repo/lib/lib.go"}
	callerOther := &model.Symbol{Name: "Service", Package: "other", File: "/repo/other/other.go"}
	callerThird := &model.Symbol{Name: "UseHelper", Package: "third", File: "/repo/third/third.go"}
	bareCallee := &model.Symbol{Name: "Helper"} // no package: bare-name index

	newCG := func(withCallback bool) *graph.CallGraph {
		cg := graph.NewCallGraph()
		if withCallback {
			cg.SetFileImportsFunc(func(file string) []string {
				if file == "/repo/other/other.go" {
					return []string{"example.com/scratch/lib"}
				}
				return nil
			})
		}
		cg.AddEdge(callerOther, bareCallee)
		cg.AddEdge(callerThird, bareCallee)
		cg.Finalize()
		return cg
	}

	names := func(callers []*model.Symbol) map[string]bool {
		m := make(map[string]bool, len(callers))
		for _, c := range callers {
			m[c.Package+"."+c.Name] = true
		}
		return m
	}

	// Nil callback: old behavior - cross-package bare-name callers dropped.
	if got := names(newCG(false).GetCallers(target)); len(got) != 0 {
		t.Errorf("nil callback: expected no callers, got %v", got)
	}

	// Callback set: other kept (imports lib), third dropped (no import).
	got := names(newCG(true).GetCallers(target))
	if !got["other.Service"] {
		t.Errorf("callback set: expected other.Service kept, got %v", got)
	}
	if got["third.UseHelper"] {
		t.Errorf("callback set: expected third.UseHelper dropped, got %v", got)
	}
}
