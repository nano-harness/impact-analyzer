package lang

import (
	"fmt"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

var (
	registry = &Registry{
		adapters: make(map[model.Language]LanguageAdapter),
	}
)

// Registry manages language adapters
type Registry struct {
	mu       sync.RWMutex
	adapters map[model.Language]LanguageAdapter
}

// Register registers a language adapter
func Register(adapter LanguageAdapter) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.adapters[adapter.Language()] = adapter
}

// Get retrieves a language adapter
func Get(lang model.Language) (LanguageAdapter, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	adapter, ok := registry.adapters[lang]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for language: %s", lang)
	}
	return adapter, nil
}

// GetAll returns all registered adapters
func GetAll() []LanguageAdapter {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	adapters := make([]LanguageAdapter, 0, len(registry.adapters))
	for _, adapter := range registry.adapters {
		adapters = append(adapters, adapter)
	}
	return adapters
}

// IsRegistered checks if an adapter is registered for the given language
func IsRegistered(l model.Language) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	_, ok := registry.adapters[l]
	return ok
}

// Available returns a list of all registered language adapters
func Available() []model.Language {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]model.Language, 0, len(registry.adapters))
	for l := range registry.adapters {
		out = append(out, l)
	}
	return out
}

// DetectLanguage attempts to detect the language of a file based on extension
func DetectLanguage(filePath string) model.Language {
	ext := getFileExtension(filePath)

	switch ext {
	case ".go":
		return model.LanguageGo
	case ".java":
		return model.LanguageJava
	case ".js", ".jsx", ".mjs", ".cjs":
		return model.LanguageJavaScript
	case ".ts", ".tsx", ".mts", ".cts":
		return model.LanguageTypeScript
	case ".rs":
		return model.LanguageRust
	case ".py", ".pyi":
		return model.LanguagePython
	default:
		return model.LanguageUnknown
	}
}

func getFileExtension(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '.' {
			return filePath[i:]
		}
		if filePath[i] == '/' || filePath[i] == '\\' {
			break
		}
	}
	return ""
}
