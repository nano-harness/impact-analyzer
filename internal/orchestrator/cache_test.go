package orchestrator

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

func TestAnalysisCacheHitMissAndExpiry(t *testing.T) {
	cache := NewAnalysisCache(t.TempDir(), true)
	key := cache.ComputeKey("base", "head", "config", "dev", CacheKeyParams{StopAtEntryPoints: true})
	if got, ok := cache.Get(key); ok || got != nil {
		t.Fatal("empty cache returned a hit")
	}

	want := &model.ImpactResult{SchemaVersion: "1.0", TotalReach: 2}
	if err := cache.Put(key, want); err != nil {
		t.Fatal(err)
	}
	got, ok := cache.Get(key)
	if !ok || got.TotalReach != want.TotalReach {
		t.Fatalf("cache hit=(%v,%v), want total reach %d", got, ok, want.TotalReach)
	}

	cache.ttl = time.Nanosecond
	time.Sleep(time.Millisecond)
	if got, ok := cache.Get(key); ok || got != nil {
		t.Fatal("expired cache entry returned a hit")
	}
}

func TestComputeConfigHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".impact.yaml")
	if hash, err := ComputeConfigHash(path); err != nil || hash != "no-config" {
		t.Fatalf("missing config hash=(%q,%v), want no-config", hash, err)
	}
}

// TestAnalysisCacheKeyIncludesDepth is a regression test: two runs over the
// same diff with different traversal depths must not share a cache entry
// (previously a max_depth=1 result was served for a later --depth 5 run).
func TestAnalysisCacheKeyIncludesDepth(t *testing.T) {
	cache := NewAnalysisCache(t.TempDir(), true)
	base := CacheKeyParams{StopAtEntryPoints: true, UseDeepAnalysis: true, WithSurface: true, TraceMaxDepth: 8}

	shallow := base
	shallow.MaxDepth = 1
	deep := base
	deep.MaxDepth = 5
	if cache.ComputeKey("base", "head", "config", "dev", shallow) == cache.ComputeKey("base", "head", "config", "dev", deep) {
		t.Fatal("cache key ignores traversal depth")
	}

	noTrace := base
	noTrace.TraceAffectedContracts = false
	trace := base
	trace.TraceAffectedContracts = true
	if cache.ComputeKey("base", "head", "config", "dev", noTrace) == cache.ComputeKey("base", "head", "config", "dev", trace) {
		t.Fatal("cache key ignores contract tracing toggle")
	}

	deepOff := base
	deepOff.UseDeepAnalysis = false
	if cache.ComputeKey("base", "head", "config", "dev", base) == cache.ComputeKey("base", "head", "config", "dev", deepOff) {
		t.Fatal("cache key ignores use_deep_analysis toggle")
	}

	dirtyA, dirtyB := base, base
	dirtyA.ContentHash = "diff-hash-a"
	dirtyB.ContentHash = "diff-hash-b"
	if cache.ComputeKey("base", "head", "config", "dev", dirtyA) == cache.ComputeKey("base", "head", "config", "dev", dirtyB) {
		t.Fatal("cache key ignores diff content hash")
	}
}

// TestAnalysisCacheNoStaleDepthHit runs two real analyses over the same diff
// with different depths against one cache: the second run must recompute and
// see deeper callers instead of receiving the first run's cached result.
func TestAnalysisCacheNoStaleDepthHit(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "main.go", `package main

func Changed() int { return 1 }

func Level1() int { return Changed() }

func Level2() int { return Level1() }

func Level3() int { return Level2() }

func main() { Level3() }
`)
	repo.Commit(t, "initial")
	repo.WriteFile(t, "main.go", `package main

func Changed() int { return 2 }

func Level1() int { return Changed() }

func Level2() int { return Level1() }

func Level3() int { return Level2() }

func main() { Level3() }
`)
	repo.Commit(t, "change")

	cfg := &config.Config{}
	cfg.Index.IncrementalMode = true
	// Deep analysis off: the mini repo has no go.mod, and tree-sitter edges
	// alone suffice for the depth assertion.
	cfg.Go.UseDeepAnalysis = false

	cacheDir := filepath.Join(t.TempDir(), "cache")
	o, err := NewOrchestratorWithOptions(repo.Dir, cacheDir, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	reachOf := func(result *model.ImpactResult) int {
		for _, ch := range result.Changes {
			if ch.Change != nil && ch.Change.Symbol != nil && ch.Change.Symbol.Name == "Changed" {
				return ch.Reach
			}
		}
		t.Fatal("no change reported for symbol Changed")
		return 0
	}

	shallow, err := o.AnalyzeDiff("HEAD~1", 1)
	if err != nil {
		t.Fatal(err)
	}
	deep, err := o.AnalyzeDiff("HEAD~1", -1)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := reachOf(shallow), 1; got != want {
		t.Fatalf("depth=1 reach=%d, want %d", got, want)
	}
	// A stale hit would return the depth-1 result here.
	if got := reachOf(deep); got <= 1 {
		t.Fatalf("depth=-1 reach=%d, want > 1 (stale cache hit?)", got)
	}
}

// TestAnalysisCacheNoStaleDirtyTreeHit is a regression test: with a dirty
// working tree, `git diff HEAD~1` covers uncommitted edits, so the cache key
// must change when the worktree content changes. Previously the key only
// hashed ref/HEAD/config, serving stale results for dirty-tree runs.
func TestAnalysisCacheNoStaleDirtyTreeHit(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "main.go", `package main

func Changed() int { return 1 }

func Level1() int { return Changed() }

func main() { Level1() }
`)
	repo.Commit(t, "initial")
	repo.WriteFile(t, "main.go", `package main

func Changed() int { return 2 }

func Level1() int { return Changed() }

func main() { Level1() }
`)
	repo.Commit(t, "change")

	cfg := &config.Config{}
	cfg.Index.IncrementalMode = true
	cfg.Go.UseDeepAnalysis = false

	cacheDir := filepath.Join(t.TempDir(), "cache")
	o, err := NewOrchestratorWithOptions(repo.Dir, cacheDir, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	reachOf := func(result *model.ImpactResult) int {
		for _, ch := range result.Changes {
			if ch.Change != nil && ch.Change.Symbol != nil && ch.Change.Symbol.Name == "Changed" {
				return ch.Reach
			}
		}
		t.Fatal("no change reported for symbol Changed")
		return 0
	}

	committed, err := o.AnalyzeDiff("HEAD~1", -1)
	if err != nil {
		t.Fatal(err)
	}

	// Dirty the worktree so Level1 no longer calls Changed: callers of
	// Changed drop to zero. A stale hit would serve the committed reach.
	repo.WriteFile(t, "main.go", `package main

func Changed() int { return 2 }

func Level1() int { return 0 }

func main() { Level1() }
`)

	dirty, err := o.AnalyzeDiff("HEAD~1", -1)
	if err != nil {
		t.Fatal(err)
	}

	if got, committedReach := reachOf(dirty), reachOf(committed); got == committedReach {
		t.Fatalf("dirty-tree reach=%d equals committed reach (stale cache hit?)", got)
	}
}
