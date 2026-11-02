package index

import (
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// IndexStore defines the interface for persistent symbol storage
type IndexStore interface {
	// StoreFileSymbols stores symbols for a file
	StoreFileSymbols(fileSymbols *model.FileSymbols) error

	// GetFileSymbols retrieves symbols for a file
	GetFileSymbols(filePath string) (*model.FileSymbols, error)

	// StoreCallSite stores a call site
	StoreCallSite(callSite *model.CallSite) error

	// GetCallSitesInFile retrieves all call sites in a file
	GetCallSitesInFile(filePath string) ([]*model.CallSite, error)

	// GetCallersOf retrieves all symbols that call the given symbol
	GetCallersOf(symbolName string) ([]*model.Symbol, error)

	// Close closes the store
	Close() error
}
