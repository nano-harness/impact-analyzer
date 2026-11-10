package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// stubDeepAnalyzer implements lang.DeepAnalyzer, counting BuildPackageCallGraph
// invocations per (dir, buildTags) key.
type stubDeepAnalyzer struct {
	invocations map[string]int
	result      []*model.CallSite
	err         error
}

func newStubDeepAnalyzer(result []*model.CallSite, err error) *stubDeepAnalyzer {
	return &stubDeepAnalyzer{invocations: make(map[string]int), result: result, err: err}
}

func (s *stubDeepAnalyzer) Language() model.Language                       { return model.LanguageGo }
func (s *stubDeepAnalyzer) ParseFile(string) (*model.FileSymbols, error)   { return nil, nil }
func (s *stubDeepAnalyzer) ExtractCalls(string) ([]*model.CallSite, error) { return nil, nil }
func (s *stubDeepAnalyzer) IsTestFile(string) bool                         { return false }
func (s *stubDeepAnalyzer) IsExported(string, interface{}) bool            { return false }
func (s *stubDeepAnalyzer) ResolveImports(string) ([]string, error)        { return nil, nil }

func (s *stubDeepAnalyzer) BuildPackageCallGraph(dir string, buildTags, _ []string) ([]*model.CallSite, error) {
	s.invocations[dir+"|"+strings.Join(buildTags, ",")]++
	return s.result, s.err
}

func (s *stubDeepAnalyzer) count(dir string, buildTags ...string) int {
	return s.invocations[dir+"|"+strings.Join(buildTags, ",")]
}

func TestGoDeepCallCache_MemoizesPerDir(t *testing.T) {
	want := []*model.CallSite{{File: "a.go", Line: 1}}
	stub := newStubDeepAnalyzer(want, nil)
	cache := newGoDeepCallCache()

	for i := 0; i < 3; i++ {
		got, err := cache.buildPackageCallGraph(stub, "/repo/pkg", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("cached result mismatch: got %v", got)
		}
	}
	if n := stub.count("/repo/pkg"); n != 1 {
		t.Fatalf("expected 1 SSA build, got %d", n)
	}
}

func TestGoDeepCallCache_CachesErrors(t *testing.T) {
	wantErr := errors.New("load failed")
	stub := newStubDeepAnalyzer(nil, wantErr)
	cache := newGoDeepCallCache()

	for i := 0; i < 2; i++ {
		_, err := cache.buildPackageCallGraph(stub, "/repo/broken", nil, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected cached error %v, got %v", wantErr, err)
		}
	}
	if n := stub.count("/repo/broken"); n != 1 {
		t.Fatalf("expected 1 SSA build for failing dir, got %d", n)
	}
}

func TestGoDeepCallCache_KeyedByDirAndTags(t *testing.T) {
	stub := newStubDeepAnalyzer(nil, nil)
	cache := newGoDeepCallCache()

	_, _ = cache.buildPackageCallGraph(stub, "/repo/a", nil, nil)
	_, _ = cache.buildPackageCallGraph(stub, "/repo/b", nil, nil)
	_, _ = cache.buildPackageCallGraph(stub, "/repo/a", []string{"race"}, nil)

	checks := []struct {
		dir  string
		tags []string
	}{
		{"/repo/a", nil},
		{"/repo/b", nil},
		{"/repo/a", []string{"race"}},
	}
	for _, c := range checks {
		if got := stub.count(c.dir, c.tags...); got != 1 {
			t.Fatalf("builds for (%s, %v) = %d, want 1", c.dir, c.tags, got)
		}
	}
}

func TestGoDeepCallCache_Clear(t *testing.T) {
	stub := newStubDeepAnalyzer(nil, nil)
	cache := newGoDeepCallCache()

	_, _ = cache.buildPackageCallGraph(stub, "/repo/pkg", nil, nil)
	cache.Clear()
	_, _ = cache.buildPackageCallGraph(stub, "/repo/pkg", nil, nil)

	if n := stub.count("/repo/pkg"); n != 2 {
		t.Fatalf("expected rebuild after Clear, got %d builds", n)
	}
}

func TestGoDeepCallCache_FilterScopeIsolation(t *testing.T) {
	cache := newGoDeepCallCache()
	counts := make(map[string]int)
	scopes := []struct {
		filtered bool
		scope    string
		label    string
	}{
		{false, "", "full"}, {true, "", "filtered-empty"}, {true, "/repo-a", "scope-a"}, {true, "/repo-b", "scope-b"},
	}
	for pass := 0; pass < 2; pass++ {
		for _, scope := range scopes {
			got, err := cache.getScoped("/repo/pkg", nil, nil, scope.filtered, scope.scope, func() ([]*model.CallSite, error) {
				counts[scope.label]++
				return []*model.CallSite{{File: scope.label}}, nil
			})
			if err != nil || len(got) != 1 || got[0].File != scope.label {
				t.Fatalf("scope %s contaminated: %v %v", scope.label, got, err)
			}
		}
	}
	for label, count := range counts {
		if count != 1 {
			t.Fatalf("%s built %d times", label, count)
		}
	}
	// The pre-existing full-cache API must hit the full entry, not any filter.
	got, err := cache.get("/repo/pkg", nil, nil, func() ([]*model.CallSite, error) { t.Fatal("full entry not reused"); return nil, nil })
	if err != nil || got[0].File != "full" {
		t.Fatal("full API contaminated")
	}
	cache.Clear()
	if len(cache.entries) != 0 {
		t.Fatal("Clear retained scoped entries")
	}
}
