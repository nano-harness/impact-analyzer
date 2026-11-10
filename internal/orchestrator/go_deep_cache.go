package orchestrator

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/lang/golang/godeep"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// goDeepCallCache memoizes DeepAnalyzer.BuildPackageCallGraph results for the
// duration of a single analysis run. A run builds the call graph in several
// passes (changed files, expanded scope, test files, supplement files) and
// every pass re-analyzes each Go directory; without caching, each of those
// passes pays a full LoadPackages + SSA build + RTA per directory even though
// the code on disk cannot change mid-run.
//
// Entries are keyed by (dir, buildTags, entryPoints) and errors are cached
// too: a directory whose SSA build fails keeps failing for the rest of the
// run instead of re-running the same expensive load 3-4 times. The cache is
// not meant to be long-lived: callers Clear it at the start of each run (same
// lifetime as lang.ParseCache). It is safe for concurrent use.
type goDeepCallCache struct {
	mu      sync.Mutex
	entries map[goDeepCallKey]goDeepCallEntry
}

type goDeepCallKey struct {
	filtered    bool
	scope       string
	dir         string
	buildTags   string
	entryPoints string
}

type goDeepCallEntry struct {
	calls []*model.CallSite
	err   error
}

func newGoDeepCallCache() *goDeepCallCache {
	return &goDeepCallCache{entries: make(map[goDeepCallKey]goDeepCallEntry)}
}

// buildPackageCallGraph returns the cached SSA/RTA call sites for a package
// directory, running the deep analyzer on first use within the run.
func (c *goDeepCallCache) buildPackageCallGraph(analyzer lang.DeepAnalyzer, dir string, buildTags, entryPoints []string) ([]*model.CallSite, error) {
	return c.get(dir, buildTags, entryPoints, func() ([]*model.CallSite, error) {
		return analyzer.BuildPackageCallGraph(dir, buildTags, entryPoints)
	})
}

// callSitesForDir returns the cached SSA/RTA call sites for a package
// directory, computing them from the shared SSA session on first use within
// the run. Cache key and lifetime are identical to buildPackageCallGraph.
func (c *goDeepCallCache) callSitesForDir(session *godeep.Session, dir string, buildTags, entryPoints []string) ([]*model.CallSite, error) {
	return c.get(dir, buildTags, entryPoints, func() ([]*model.CallSite, error) {
		return session.CallSitesForDir(dir, entryPoints...)
	})
}

// callSitesForDirFiltered keeps filtered results separate from full results and
// other scopes. The caller must choose a scope identifying the predicate's
// semantics, including its root; function pointer identity is not sufficient.
func (c *goDeepCallCache) callSitesForDirFiltered(session *godeep.Session, dir string, buildTags, entryPoints []string, scope string, keep func(string) bool) ([]*model.CallSite, error) {
	return c.getScoped(dir, buildTags, entryPoints, true, scope, func() ([]*model.CallSite, error) {
		return session.CallSitesForDirFiltered(dir, keep, entryPoints...)
	})
}

// get memoizes a per-directory call-site build under the (dir, buildTags,
// entryPoints) key.
func (c *goDeepCallCache) get(dir string, buildTags, entryPoints []string, build func() ([]*model.CallSite, error)) ([]*model.CallSite, error) {
	return c.getScoped(dir, buildTags, entryPoints, false, "", build)
}

func (c *goDeepCallCache) getScoped(dir string, buildTags, entryPoints []string, filtered bool, scope string, build func() ([]*model.CallSite, error)) ([]*model.CallSite, error) {
	key := goDeepCallKey{
		filtered:    filtered,
		scope:       scope,
		dir:         dir,
		buildTags:   strings.Join(buildTags, ","),
		entryPoints: strings.Join(entryPoints, ","),
	}

	c.mu.Lock()
	entry, ok := c.entries[key]
	c.mu.Unlock()
	if ok {
		if os.Getenv("IMPACT_VERBOSE") != "" {
			fmt.Fprintf(os.Stderr, "[ssa] reusing cached call graph for %s\n", dir)
		}
		return entry.calls, entry.err
	}

	if os.Getenv("IMPACT_VERBOSE") != "" {
		fmt.Fprintf(os.Stderr, "[ssa] building call graph for %s\n", dir)
	}
	calls, err := build()

	c.mu.Lock()
	c.entries[key] = goDeepCallEntry{calls: calls, err: err}
	c.mu.Unlock()
	return calls, err
}

// Clear drops all cached entries. Callers invoke it at the start of each
// analysis run to bound the cache lifetime to a single run.
func (c *goDeepCallCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[goDeepCallKey]goDeepCallEntry)
	c.mu.Unlock()
}
