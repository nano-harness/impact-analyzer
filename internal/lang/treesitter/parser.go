package treesitter

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tsjavascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// queryCacheKey identifies a compiled query by language and source.
// sitter.Query binds only to the language, not to a parser or tree, so
// compiled queries can be shared across parser instances. grammar
// distinguishes variants sharing one model.Language (TS vs TSX).
type queryCacheKey struct {
	lang    model.Language
	grammar string
	query   string
}

// queryCacheEntry holds a compiled query or the error from compiling it.
// Errors are cached too, to avoid recompiling known-bad query sources.
type queryCacheEntry struct {
	query *sitter.Query
	err   error
}

// queryCache caches compiled Tree-sitter queries for the process lifetime.
// Cached queries are never closed.
var queryCache sync.Map // map[queryCacheKey]queryCacheEntry

// Parser wraps a Tree-sitter parser for a specific language
type Parser struct {
	language *sitter.Language
	parser   *sitter.Parser
	lang     model.Language

	// TypeScript ships two grammars: plain TS and TSX. tsxParser/tsxLanguage
	// are only set for LanguageTypeScript; ParseFile picks the TSX variant
	// for .tsx files. lastUsedTSX records the grammar of the most recent
	// Parse call so Query compiles against the matching grammar.
	tsxLanguage *sitter.Language
	tsxParser   *sitter.Parser
	lastUsedTSX bool
}

// NewParser creates a new Tree-sitter parser for the given language
func NewParser(lang model.Language) (*Parser, error) {
	language, err := loadLanguage(lang)
	if err != nil {
		return nil, err
	}

	parser := sitter.NewParser()
	if err := parser.SetLanguage(language); err != nil {
		return nil, fmt.Errorf("failed to set parser language for %s: %w", lang, err)
	}

	p := &Parser{
		language: language,
		parser:   parser,
		lang:     lang,
	}

	if lang == model.LanguageTypeScript {
		tsxLanguage := sitter.NewLanguage(tsts.LanguageTSX())
		tsxParser := sitter.NewParser()
		if err := tsxParser.SetLanguage(tsxLanguage); err != nil {
			return nil, fmt.Errorf("failed to set TSX parser language: %w", err)
		}
		p.tsxLanguage = tsxLanguage
		p.tsxParser = tsxParser
	}

	return p, nil
}

// ParseFile parses a file and returns the syntax tree
func (p *Parser) ParseFile(filePath string) (*sitter.Tree, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	p.lastUsedTSX = p.tsxParser != nil && strings.HasSuffix(filePath, ".tsx")
	parser := p.parser
	if p.lastUsedTSX {
		parser = p.tsxParser
	}

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse file: %s", filePath)
	}

	return tree, nil
}

// ParseContent parses source code content
func (p *Parser) ParseContent(content []byte) (*sitter.Tree, error) {
	p.lastUsedTSX = false
	tree := p.parser.Parse(content, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse content")
	}
	return tree, nil
}

// Query executes a Tree-sitter query on a syntax tree
func (p *Parser) Query(tree *sitter.Tree, querySource string) ([]*QueryMatch, error) {
	language := p.language
	grammar := ""
	if p.lastUsedTSX {
		if p.tsxLanguage == nil {
			return nil, fmt.Errorf("TSX parser language not initialized for %s", p.lang)
		}
		language = p.tsxLanguage
		grammar = "tsx"
	}
	if language == nil {
		return nil, fmt.Errorf("parser language not initialized for %s", p.lang)
	}

	cacheKey := queryCacheKey{lang: p.lang, grammar: grammar, query: querySource}
	if cached, ok := queryCache.Load(cacheKey); ok {
		entry := cached.(queryCacheEntry)
		if entry.err != nil {
			return nil, fmt.Errorf("failed to create query: %w", entry.err)
		}
		return p.runQuery(entry.query, tree)
	}

	query, queryErr := sitter.NewQuery(language, querySource)
	entry := queryCacheEntry{query: query}
	if queryErr != nil {
		// NewQuery returns *sitter.QueryError; only assign when non-nil so
		// the interface field does not hold a typed nil.
		entry.err = queryErr
	}
	if existing, loaded := queryCache.LoadOrStore(cacheKey, entry); loaded {
		// Another goroutine compiled it first; use the cached one and
		// release the duplicate we just created.
		if query != nil {
			query.Close()
		}
		entry = existing.(queryCacheEntry)
	}
	if entry.err != nil {
		return nil, fmt.Errorf("failed to create query: %w", entry.err)
	}
	return p.runQuery(entry.query, tree)
}

// runQuery executes a compiled (cached) query on a syntax tree.
// The query is owned by the cache and must not be closed here.
func (p *Parser) runQuery(query *sitter.Query, tree *sitter.Tree) ([]*QueryMatch, error) {
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	qms := cursor.Matches(query, tree.RootNode(), []byte{})
	captureNames := query.CaptureNames()

	var matches []*QueryMatch
	for {
		match := qms.Next()
		if match == nil {
			break
		}

		qm := &QueryMatch{
			PatternIndex: match.PatternIndex,
			Captures:     make(map[string]*sitter.Node),
		}

		for i := range match.Captures {
			capture := &match.Captures[i]
			if int(capture.Index) < len(captureNames) {
				node := capture.Node
				qm.Captures[captureNames[capture.Index]] = &node
			}
		}

		matches = append(matches, qm)
	}

	return matches, nil
}

// QueryMatch represents a single match from a Tree-sitter query
type QueryMatch struct {
	PatternIndex uint
	Captures     map[string]*sitter.Node
}

// GetNodeText extracts the text content of a node
func GetNodeText(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	return string(source[node.StartByte():node.EndByte()])
}

func loadLanguage(lang model.Language) (*sitter.Language, error) {
	switch lang {
	case model.LanguageGo:
		return sitter.NewLanguage(tsgo.Language()), nil
	case model.LanguageJava:
		return sitter.NewLanguage(tsjava.Language()), nil
	case model.LanguageJavaScript:
		return sitter.NewLanguage(tsjavascript.Language()), nil
	case model.LanguageTypeScript:
		return sitter.NewLanguage(tsts.LanguageTypescript()), nil
	case model.LanguageRust:
		return sitter.NewLanguage(tsrust.Language()), nil
	case model.LanguagePython:
		return sitter.NewLanguage(tspython.Language()), nil
	default:
		return nil, fmt.Errorf("unsupported language %q", string(lang))
	}
}
