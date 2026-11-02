package treesitter

import (
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ParserPool provides thread-safe pooling of Tree-sitter parsers
// to avoid the overhead of parser initialization for every file
type ParserPool struct {
	pools map[model.Language]*sync.Pool
	mu    sync.RWMutex
}

var globalPool = &ParserPool{
	pools: make(map[model.Language]*sync.Pool),
}

// GetParser retrieves a parser from the pool or creates a new one
func GetParser(lang model.Language) (*Parser, error) {
	return globalPool.get(lang)
}

// PutParser returns a parser to the pool for reuse
func PutParser(p *Parser) {
	if p != nil {
		globalPool.put(p)
	}
}

func (pp *ParserPool) get(lang model.Language) (*Parser, error) {
	pp.mu.RLock()
	pool, exists := pp.pools[lang]
	pp.mu.RUnlock()

	if !exists {
		pp.mu.Lock()
		// Double-check after acquiring write lock
		if _, exists := pp.pools[lang]; !exists {
			pp.pools[lang] = &sync.Pool{
				New: func() interface{} {
					// This will be called when pool is empty
					parser, _ := NewParser(lang)
					return parser
				},
			}
		}
		pool = pp.pools[lang]
		pp.mu.Unlock()
	}

	parser := pool.Get().(*Parser)
	if parser == nil {
		// Fallback in case pool.New returned nil due to error
		return NewParser(lang)
	}
	return parser, nil
}

func (pp *ParserPool) put(p *Parser) {
	pp.mu.RLock()
	pool, exists := pp.pools[p.lang]
	pp.mu.RUnlock()

	if exists {
		pool.Put(p)
	}
}

// WithParser executes a function with a pooled parser, automatically returning it
func WithParser(lang model.Language, fn func(*Parser) error) error {
	parser, err := GetParser(lang)
	if err != nil {
		return err
	}
	defer PutParser(parser)
	return fn(parser)
}
