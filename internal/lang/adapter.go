package lang

import (
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// LanguageAdapter defines the interface that all language adapters must implement
type LanguageAdapter interface {
	// Language returns the language this adapter handles
	Language() model.Language

	// ParseFile extracts symbols from a single file
	ParseFile(filePath string) (*model.FileSymbols, error)

	// ExtractCalls extracts call sites from a file
	ExtractCalls(filePath string) ([]*model.CallSite, error)

	// IsTestFile determines if a file is a test file based on language conventions
	IsTestFile(filePath string) bool

	// IsExported determines if a symbol is exported based on language rules
	IsExported(symbolName string, context interface{}) bool

	// ResolveImports resolves import statements in a file
	ResolveImports(filePath string) ([]string, error)
}

// DeepAnalyzer is an optional interface for languages that support deep semantic analysis
// Currently only implemented for Go (using go/types + go/callgraph)
type DeepAnalyzer interface {
	LanguageAdapter

	// BuildPackageCallGraph builds precise call edges for a package using type information (#21)
	// pkgDir: directory containing the package to analyze
	// buildTags: optional build tags to use during analysis
	// customEntryPoints: optional list of qualified function names to use as entry points
	// Returns call sites discovered through SSA/RTA analysis
	BuildPackageCallGraph(pkgDir string, buildTags, customEntryPoints []string) ([]*model.CallSite, error)
}

// ParserOptions contains options for parsing
type ParserOptions struct {
	IncludeComments bool
	IncludePrivate  bool
	MaxFileSize     int64 // Skip files larger than this (bytes)
	FollowImports   bool
}
