package astutil

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// parseCacheKey identifies a parsed file by its path and on-disk identity
// (modification time and size), so a file rewritten between lookups produces
// a different key and is re-parsed instead of serving a stale tree.
type parseCacheKey struct {
	path  string
	mtime int64
	size  int64
}

// parseCacheEntry owns the products of one tree-sitter parse. The tree stays
// open until ClearParseCache so that several same-language extractors can
// walk the same file without each paying for a parse.
type parseCacheEntry struct {
	parser  *treesitter.Parser
	tree    *sitter.Tree
	content []byte
	lang    model.Language
}

// parseCache deduplicates tree-sitter parses across extractors within one
// index build / batch load. A batch may call ParseContent concurrently from
// its worker pool: the mutex guards the map, entries are read-only once
// stored, and a duplicate concurrent parse of the same file is released in
// favor of the first entry. Whole batches (BuildIndex, the git loader's
// working-tree loads) are serialized by the contract package's batch mutex,
// because a batch's deferred ClearParseCache closes trees another batch
// could still be walking.
var parseCache = struct {
	sync.Mutex
	entries map[parseCacheKey]*parseCacheEntry
	// parses counts real tree-sitter parses (cache misses) since the last
	// ClearParseCache. Test/diagnostic instrumentation, see RealParseCount.
	parses int
}{entries: make(map[parseCacheKey]*parseCacheEntry)}

// ClearParseCache closes every cached tree and empties the cache. Callers
// that run a batch of extractions (contract.BuildIndex, the git loader's
// loadContractsFromWorkingTree) defer it once per batch so cached trees never
// outlive the batch. The wrapped parsers have no Close in this codebase and
// stay GC-managed once unreferenced, exactly as before the cache existed.
func ClearParseCache() {
	parseCache.Lock()
	defer parseCache.Unlock()
	for key, entry := range parseCache.entries {
		entry.tree.Close()
		delete(parseCache.entries, key)
	}
	parseCache.parses = 0
}

// RealParseCount reports how many real tree-sitter parses (cache misses) ran
// since the last ClearParseCache. Instrumentation for tests and diagnostics;
// production code must not depend on it.
func RealParseCount() int {
	parseCache.Lock()
	defer parseCache.Unlock()
	return parseCache.parses
}

// ParseFile parses a source file with the language inferred from its extension.
func ParseFile(filePath string) (*treesitter.Parser, *sitter.Tree, []byte, model.Language, error) {
	return ParseContent(filePath, nil)
}

// ParseContent is the content-based variant of ParseFile: it parses pre-read
// content instead of reading the file from disk, so callers that already
// loaded the file (e.g. index building) avoid a duplicate read. A nil content
// falls back to reading the file from disk, exactly like ParseFile.
//
// The parse goes through a per-build cache keyed by (path, mtime, size): a
// second call for the same unchanged file returns the same tree without
// re-parsing, so several extractors over one file parse it only once. The
// caller-provided content is assumed to come from the file currently on disk.
//
// Ownership: the returned tree is shared and owned by the cache; callers must
// NOT Close it. Trees are only read (walked/queried), never mutated, so
// sharing is safe. ClearParseCache releases all cached trees.
func ParseContent(filePath string, content []byte) (*treesitter.Parser, *sitter.Tree, []byte, model.Language, error) {
	lang := LanguageForPath(filePath)

	key := parseCacheKey{path: filePath, mtime: 0, size: -1}
	if info, statErr := os.Stat(filePath); statErr == nil {
		key.mtime = info.ModTime().UnixNano()
		key.size = info.Size()
	}

	parseCache.Lock()
	if entry, ok := parseCache.entries[key]; ok {
		parseCache.Unlock()
		return entry.parser, entry.tree, entry.content, entry.lang, nil
	}
	parseCache.Unlock()

	parser, err := treesitter.NewParser(lang)
	if err != nil {
		return nil, nil, nil, lang, err
	}
	if content == nil {
		content, err = osReadFile(filePath)
		if err != nil {
			return nil, nil, nil, lang, err
		}
	}
	tree, err := parser.ParseContent(content)
	if err != nil {
		return nil, nil, nil, lang, err
	}

	parseCache.Lock()
	if existing, ok := parseCache.entries[key]; ok {
		// Lost a concurrent parse race: keep the first entry and release the
		// duplicate tree instead of leaking it.
		parseCache.Unlock()
		tree.Close()
		return existing.parser, existing.tree, existing.content, existing.lang, nil
	}
	parseCache.entries[key] = &parseCacheEntry{parser: parser, tree: tree, content: content, lang: lang}
	parseCache.parses++
	parseCache.Unlock()
	return parser, tree, content, lang, nil
}

var osReadFile = func(name string) ([]byte, error) { return os.ReadFile(name) }

func LanguageForPath(filePath string) model.Language {
	switch filepath.Ext(filePath) {
	case ".ts", ".tsx":
		return model.LanguageTypeScript
	case ".java":
		return model.LanguageJava
	case ".rs":
		return model.LanguageRust
	case ".py", ".pyi":
		return model.LanguagePython
	default:
		return model.LanguageJavaScript
	}
}

func Text(node *sitter.Node, source []byte) string {
	return treesitter.GetNodeText(node, source)
}

func Walk(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := uint(0); i < node.NamedChildCount(); i++ {
		Walk(node.NamedChild(i), fn)
	}
}

func NamedChildren(node *sitter.Node) []*sitter.Node {
	if node == nil {
		return nil
	}
	children := make([]*sitter.Node, 0, node.NamedChildCount())
	for i := uint(0); i < node.NamedChildCount(); i++ {
		children = append(children, node.NamedChild(i))
	}
	return children
}

func ArgNode(args *sitter.Node, index int) *sitter.Node {
	if args == nil || args.Kind() != "arguments" {
		return nil
	}
	seen := 0
	for i := uint(0); i < args.NamedChildCount(); i++ {
		child := args.NamedChild(i)
		if child.Kind() == "comment" {
			continue
		}
		if seen == index {
			return child
		}
		seen++
	}
	return nil
}

func StringValue(node *sitter.Node, source []byte) (string, bool) {
	if node == nil {
		return "", false
	}
	if node.Kind() != "string" && node.Kind() != "string_literal" {
		return "", false
	}
	text := Text(node, source)
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"'`")
	return text, true
}

func FirstStringArg(args *sitter.Node, source []byte) (string, bool) {
	return StringValue(ArgNode(args, 0), source)
}

func StringLiterals(node *sitter.Node, source []byte) []string {
	var values []string
	Walk(node, func(n *sitter.Node) {
		if value, ok := StringValue(n, source); ok {
			values = append(values, value)
		}
	})
	return values
}

func JoinPath(prefix, path string) string {
	if prefix == "" && path == "" {
		return "/"
	}
	trim := func(s string) string { return strings.Trim(s, "/") }
	parts := make([]string, 0, 2)
	if p := trim(prefix); p != "" {
		parts = append(parts, p)
	}
	if p := trim(path); p != "" {
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}

func IsOutermostCall(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return true
	}
	return !(parent.Kind() == "member_expression" || parent.Kind() == "field_expression")
}
