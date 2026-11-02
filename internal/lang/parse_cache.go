package lang

import (
	"os"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ParseCache memoizes LanguageAdapter results for the duration of a single
// analysis run. Within one run the same file is parsed from several phases
// (call graph build, symbol diff, package derivation, import indexing, ...);
// caching avoids re-running tree-sitter on unchanged content.
//
// Entries are keyed by (path, mtime, size) so a stale result is never served
// for a file that changed on disk. The cache is not meant to be long-lived:
// callers create or Clear it at the start of each run. It is safe for
// concurrent use.
type ParseCache struct {
	mu      sync.Mutex
	entries map[parseCacheKey]parseCacheEntry
	calls   map[parseCacheKey]callsEntry
	imports map[parseCacheKey]importsEntry
}

type parseCacheKey struct {
	path  string
	mtime int64
	size  int64
}

type parseCacheEntry struct {
	symbols *model.FileSymbols
	err     error
}

type callsEntry struct {
	calls []*model.CallSite
	err   error
}

type importsEntry struct {
	imports []string
	err     error
}

// NewParseCache creates an empty parse cache.
func NewParseCache() *ParseCache {
	return &ParseCache{entries: make(map[parseCacheKey]parseCacheEntry)}
}

// statKey builds the invalidation key for filePath. The second return value
// is false when the file cannot be stated, in which case nothing should be
// cached (no reliable invalidation key).
func statKey(filePath string) (parseCacheKey, bool) {
	info, err := os.Stat(filePath)
	if err != nil {
		return parseCacheKey{}, false
	}
	return parseCacheKey{path: filePath, mtime: info.ModTime().UnixNano(), size: info.Size()}, true
}

// Parse returns the cached ParseFile result for filePath, parsing on first
// use. Errors are cached as well: a file that fails to parse keeps failing
// for the rest of the run.
func (c *ParseCache) Parse(adapter LanguageAdapter, filePath string) (*model.FileSymbols, error) {
	key, ok := statKey(filePath)
	if !ok {
		return adapter.ParseFile(filePath)
	}

	c.mu.Lock()
	entry, hit := c.entries[key]
	c.mu.Unlock()
	if hit {
		return entry.symbols, entry.err
	}

	symbols, err := adapter.ParseFile(filePath)

	c.mu.Lock()
	c.entries[key] = parseCacheEntry{symbols: symbols, err: err}
	c.mu.Unlock()
	return symbols, err
}

// ExtractCalls returns the cached ExtractCalls result for filePath, running
// the adapter on first use. The returned slice is shared with the cache;
// callers may enrich CallSite callee fields idempotently (as buildCallGraph
// does) but must not reorder or truncate it.
func (c *ParseCache) ExtractCalls(adapter LanguageAdapter, filePath string) ([]*model.CallSite, error) {
	key, ok := statKey(filePath)
	if !ok {
		return adapter.ExtractCalls(filePath)
	}

	c.mu.Lock()
	entry, hit := c.calls[key]
	c.mu.Unlock()
	if hit {
		return entry.calls, entry.err
	}

	calls, err := adapter.ExtractCalls(filePath)

	c.mu.Lock()
	if c.calls == nil {
		c.calls = make(map[parseCacheKey]callsEntry)
	}
	c.calls[key] = callsEntry{calls: calls, err: err}
	c.mu.Unlock()
	return calls, err
}

// ResolveImports returns the cached ResolveImports result for filePath,
// running the adapter on first use. The returned slice is a copy, so callers
// may mutate it freely.
func (c *ParseCache) ResolveImports(adapter LanguageAdapter, filePath string) ([]string, error) {
	key, ok := statKey(filePath)
	if !ok {
		return adapter.ResolveImports(filePath)
	}

	c.mu.Lock()
	entry, hit := c.imports[key]
	c.mu.Unlock()
	if hit {
		return append([]string(nil), entry.imports...), entry.err
	}

	imports, err := adapter.ResolveImports(filePath)

	c.mu.Lock()
	if c.imports == nil {
		c.imports = make(map[parseCacheKey]importsEntry)
	}
	c.imports[key] = importsEntry{imports: imports, err: err}
	c.mu.Unlock()
	return append([]string(nil), imports...), err
}

// Clear drops all cached entries. Callers invoke it at the start of each
// analysis run to bound the cache lifetime to a single run.
func (c *ParseCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[parseCacheKey]parseCacheEntry)
	c.calls = nil
	c.imports = nil
	c.mu.Unlock()
}
