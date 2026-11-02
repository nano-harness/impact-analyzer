package contract

import (
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ContractExtractor defines the interface for extracting public contracts from code
// Extractors serve dual purposes:
// 1. Extract contracts for surface diff analysis
// 2. Identify entry points for call graph traversal
type ContractExtractor interface {
	// Name returns the name of the extractor (e.g., "cobra", "gin", "grpc")
	Name() string

	// Kind returns the contract kind this extractor produces
	Kind() model.ContractKind

	// Language returns the programming language this extractor supports
	Language() model.Language

	// CanHandle returns true if this extractor can process the given file
	// Typically checks file extension and/or file content (imports, annotations, etc.)
	CanHandle(filePath string) bool

	// Extract extracts all contracts from the given file
	// Returns an empty slice if no contracts are found
	Extract(filePath string) ([]*model.Contract, error)

	// IsEntryPoint returns true if the given symbol represents an entry point
	// Used for call graph traversal to stop at framework-defined boundaries
	// Example: cobra command handlers, gin HTTP handlers, gRPC service methods
	IsEntryPoint(symbol *model.Symbol) bool
}

// Framework-specific extractor marker interfaces
// These allow grouping extractors by framework family for future extensions

// CLIExtractor marks extractors that handle CLI frameworks
type CLIExtractor interface {
	ContractExtractor
	// Future: ExtractCommands() for more specialized CLI operations
}

// HTTPExtractor marks extractors that handle HTTP frameworks
type HTTPExtractor interface {
	ContractExtractor
	// Future: ExtractRoutes() for more specialized HTTP operations
}

// RPCExtractor marks extractors that handle RPC frameworks
type RPCExtractor interface {
	ContractExtractor
	// Future: ExtractServices() for more specialized RPC operations
}
