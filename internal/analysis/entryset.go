package analysis

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// EntrySetBuilder builds the set of symbols considered application entry points.
type EntrySetBuilder struct {
	cfg        config.OrphanConfig
	extractors []contract.ContractExtractor
}

func NewEntrySetBuilder(cfg config.OrphanConfig, extractors []contract.ContractExtractor) *EntrySetBuilder {
	return &EntrySetBuilder{cfg: cfg, extractors: extractors}
}

func (b *EntrySetBuilder) Build(cg *graph.CallGraph) map[string]*model.Symbol {
	entries := make(map[string]*model.Symbol)
	symbols := cg.Symbols()
	contractHandlers := b.contractHandlers(symbols)
	for _, sym := range symbols {
		if sym == nil || sym.File == "" {
			continue
		}
		if b.isEntry(sym, contractHandlers) {
			entries[sym.QualifiedName()] = sym
		}
	}
	return entries
}

func (b *EntrySetBuilder) isEntry(sym *model.Symbol, contractHandlers map[string]bool) bool {
	if contractHandlers[sym.Name] || contractHandlers[sym.QualifiedName()] {
		return true
	}
	for _, extractor := range b.extractors {
		if extractor.Language() == sym.Language && extractor.IsEntryPoint(sym) {
			return true
		}
	}
	if b.matchesExtraEntry(sym) {
		return true
	}
	if b.cfg.Heuristics.EnableInit && (sym.Name == "main" || sym.Name == "init" || sym.Name == "__main__") {
		return true
	}
	if b.cfg.Heuristics.EnableTestEntry && sym.IsTest {
		return true
	}
	if b.cfg.Heuristics.EnableFilename {
		base := strings.ToLower(filepath.Base(sym.File))
		if strings.Contains(base, "cron") || strings.Contains(base, "job") || strings.Contains(base, "handler") || strings.Contains(base, "route") {
			return true
		}
	}
	return false
}

func (b *EntrySetBuilder) contractHandlers(symbols []*model.Symbol) map[string]bool {
	handlers := make(map[string]bool)
	files := make(map[string]bool)
	for _, sym := range symbols {
		if sym != nil && sym.File != "" {
			files[sym.File] = true
		}
	}
	for file := range files {
		for _, extractor := range b.extractors {
			if !extractor.CanHandle(file) {
				continue
			}
			contracts, err := extractor.Extract(file)
			if err != nil {
				continue
			}
			for _, c := range contracts {
				for _, key := range []string{"handler", "function", "method", "task", "view", "command"} {
					if value, ok := c.Signature[key].(string); ok && value != "" && value != "anonymous" {
						handlers[value] = true
						parts := strings.Split(value, ".")
						handlers[parts[len(parts)-1]] = true
					}
				}
			}
		}
	}
	return handlers
}

func (b *EntrySetBuilder) matchesExtraEntry(sym *model.Symbol) bool {
	for _, entry := range b.cfg.ExtraEntries {
		switch v := entry.(type) {
		case string:
			if sym.QualifiedName() == v || sym.Name == v {
				return true
			}
		case map[string]interface{}:
			if matchEntryMap(sym, v) {
				return true
			}
		case map[interface{}]interface{}:
			m := make(map[string]interface{}, len(v))
			for k, val := range v {
				m[fmt.Sprint(k)] = val
			}
			if matchEntryMap(sym, m) {
				return true
			}
		}
	}
	return false
}

func matchEntryMap(sym *model.Symbol, m map[string]interface{}) bool {
	if pattern, ok := m["pattern"].(string); ok && pattern != "" {
		if matched, _ := regexp.MatchString(pattern, sym.QualifiedName()); matched {
			return true
		}
		if matched, _ := regexp.MatchString(pattern, sym.Name); matched {
			return true
		}
	}
	if pattern, ok := m["file_pattern"].(string); ok && pattern != "" {
		file := filepath.ToSlash(sym.File)
		if matched, _ := doublestar.PathMatch(filepath.ToSlash(pattern), file); matched {
			return true
		}
	}
	return false
}
