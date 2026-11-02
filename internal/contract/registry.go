package contract

import (
	"fmt"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// Global registry for contract extractors
var (
	registry = &Registry{
		extractors: make(map[string]ContractExtractor),
	}
)

// Registry manages registered contract extractors
type Registry struct {
	mu         sync.RWMutex
	extractors map[string]ContractExtractor
}

// Register registers a contract extractor
// Typically called from extractor package init() functions
func Register(extractor ContractExtractor) {
	registry.Register(extractor)
}

// Get retrieves an extractor by name
func Get(name string) (ContractExtractor, error) {
	return registry.Get(name)
}

// GetByKind retrieves all extractors that produce the given contract kind
func GetByKind(kind model.ContractKind) []ContractExtractor {
	return registry.GetByKind(kind)
}

// GetByLanguage retrieves all extractors for the given language
func GetByLanguage(lang model.Language) []ContractExtractor {
	return registry.GetByLanguage(lang)
}

// GetForFile returns all extractors that can handle the given file
func GetForFile(filePath string) []ContractExtractor {
	return registry.GetForFile(filePath)
}

// All returns all registered extractors
func All() []ContractExtractor {
	return registry.All()
}

// Register registers an extractor in the registry
func (r *Registry) Register(extractor ContractExtractor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := extractor.Name()
	if _, exists := r.extractors[name]; exists {
		// Allow re-registration for testing purposes
		// but log in verbose mode
	}
	r.extractors[name] = extractor
}

// Get retrieves an extractor by name
func (r *Registry) Get(name string) (ContractExtractor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	extractor, exists := r.extractors[name]
	if !exists {
		return nil, fmt.Errorf("contract extractor %q not registered", name)
	}
	return extractor, nil
}

// GetByKind retrieves all extractors that produce the given contract kind
func (r *Registry) GetByKind(kind model.ContractKind) []ContractExtractor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ContractExtractor
	for _, extractor := range r.extractors {
		if extractor.Kind() == kind {
			result = append(result, extractor)
		}
	}
	return result
}

// GetByLanguage retrieves all extractors for the given language
func (r *Registry) GetByLanguage(lang model.Language) []ContractExtractor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ContractExtractor
	for _, extractor := range r.extractors {
		if extractor.Language() == lang {
			result = append(result, extractor)
		}
	}
	return result
}

// GetForFile returns all extractors that can handle the given file
func (r *Registry) GetForFile(filePath string) []ContractExtractor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ContractExtractor
	for _, extractor := range r.extractors {
		if extractor.CanHandle(filePath) {
			result = append(result, extractor)
		}
	}
	return result
}

// All returns all registered extractors
func (r *Registry) All() []ContractExtractor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ContractExtractor, 0, len(r.extractors))
	for _, extractor := range r.extractors {
		result = append(result, extractor)
	}
	return result
}

// IsRegistered checks if an extractor with the given name is registered
func (r *Registry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.extractors[name]
	return exists
}

// Count returns the number of registered extractors
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.extractors)
}
