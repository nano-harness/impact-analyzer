package resolver

// ImportResolver defines the interface for resolving import paths to actual file paths
type ImportResolver interface {
	// ResolveImport resolves an import path to an actual file path
	// importPath: the import string (e.g., "./utils", "react", "com.example.Service")
	// fromFile: the absolute path of the file containing the import
	// Returns: the resolved absolute file path, or error if not found
	ResolveImport(importPath string, fromFile string) (string, error)
}
