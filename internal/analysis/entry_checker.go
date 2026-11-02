package analysis

import (
	"log"
	"os"

	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// CompositeEntryChecker aggregates multiple contract extractors to check entry points
// It delegates to each extractor's IsEntryPoint method based on language matching
type CompositeEntryChecker struct {
	extractors []contract.ContractExtractor
}

// NewCompositeEntryChecker creates a new composite entry point checker
func NewCompositeEntryChecker(extractors []contract.ContractExtractor) *CompositeEntryChecker {
	return &CompositeEntryChecker{
		extractors: extractors,
	}
}

// IsEntryPoint checks if a symbol is an entry point by consulting all extractors
func (c *CompositeEntryChecker) IsEntryPoint(sym *model.Symbol) bool {
	if sym == nil {
		return false
	}

	verbose := os.Getenv("IMPACT_VERBOSE") != ""

	// Check each extractor that matches the symbol's language
	for _, extractor := range c.extractors {
		if extractor.Language() == sym.Language && extractor.IsEntryPoint(sym) {
			if verbose {
				log.Printf("[EntryChecker] Symbol %s matched entry point pattern from %s extractor\n", sym.QualifiedName(), extractor.Name())
			}
			return true
		}
	}

	return false
}
