package model

// SchemaVersion represents the current schema version (#27)
// Increment this when making breaking changes to Symbol/CallSite/etc.
const SchemaVersion = "2.0.0" // Symbol.Owner field added

// Language represents a supported programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguageJava       Language = "java"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageRust       Language = "rust"
	LanguagePython     Language = "python"
	LanguageUnknown    Language = "unknown"
)

// SymbolKind represents the kind of a code symbol
type SymbolKind string

const (
	SymbolKindFunction  SymbolKind = "function"
	SymbolKindMethod    SymbolKind = "method"
	SymbolKindClass     SymbolKind = "class"
	SymbolKindInterface SymbolKind = "interface"
	SymbolKindStruct    SymbolKind = "struct"
	SymbolKindVariable  SymbolKind = "variable"
	SymbolKindConstant  SymbolKind = "constant"
	SymbolKindPackage   SymbolKind = "package"
	SymbolKindModule    SymbolKind = "module"
)

// CoverageStatus represents the coverage status of a symbol
type CoverageStatus string

const (
	CoverageStatusCovered   CoverageStatus = "covered"
	CoverageStatusUncovered CoverageStatus = "uncovered"
	CoverageStatusPartial   CoverageStatus = "partial"
	CoverageStatusUnknown   CoverageStatus = "" // Default when coverage is not provided
)

// EdgeKind represents the kind of relationship between two symbols.
type EdgeKind string

const (
	EdgeKindCall        EdgeKind = "call"
	EdgeKindTypeRef     EdgeKind = "type_ref"
	EdgeKindInheritance EdgeKind = "inheritance" // #22: class/interface inheritance
	EdgeKindDecorator   EdgeKind = "decorator"   // #25: Python decorator usage
)

// Symbol represents a code symbol (function, method, class, etc.)
type Symbol struct {
	Name string `json:"name"`
	// #11: enclosing class/receiver type. On callee symbols extracted from
	// call sites it instead carries the best-effort receiver hint (the
	// receiver variable/field name, not a resolved type).
	Owner          string         `json:"owner,omitempty"`
	Kind           SymbolKind     `json:"kind"`
	File           string         `json:"file"`
	Line           int            `json:"line"`
	EndLine        int            `json:"end_line,omitempty"` // End line of the symbol definition
	Column         int            `json:"column"`
	Language       Language       `json:"language"`
	Package        string         `json:"package,omitempty"`
	Signature      string         `json:"signature,omitempty"`
	TypeParameters []string       `json:"type_parameters,omitempty"` // Generic type parameters like <T, U>
	IsExported     bool           `json:"is_exported"`
	IsTest         bool           `json:"is_test"`
	DocComment     string         `json:"doc_comment,omitempty"`
	CoverageStatus CoverageStatus `json:"coverage_status,omitempty"` // Coverage annotation
}

// QualifiedName returns the fully qualified name of the symbol (#11: uses Owner)
func (s *Symbol) QualifiedName() string {
	base := s.Name
	if s.Owner != "" {
		base = s.Owner + "." + s.Name
	}
	if s.Package != "" {
		return s.Package + "." + base
	}
	return base
}

// FileSymbols represents all symbols in a file
type FileSymbols struct {
	File        string    `json:"file"`
	Language    Language  `json:"language"`
	Package     string    `json:"package,omitempty"`
	Symbols     []*Symbol `json:"symbols"`
	Imports     []string  `json:"imports"`
	ContentHash string    `json:"content_hash,omitempty"` // #15: SHA256 hash for file-level caching
}

// CallSite represents a function/method call location
type CallSite struct {
	Caller   *Symbol  `json:"caller"`
	Callee   *Symbol  `json:"callee"`
	EdgeKind EdgeKind `json:"edge_kind,omitempty"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Resolved bool     `json:"resolved"` // false for dynamic calls in JS/TS
}

// CallEdge represents a lightweight caller-callee edge from SSA analysis.
type CallEdge struct {
	CallerFile string `json:"caller_file"`
	CallerLine int    `json:"caller_line"`
	CallerName string `json:"caller_name"`
	CalleeFile string `json:"callee_file"`
	CalleeLine int    `json:"callee_line"`
	CalleeName string `json:"callee_name"`
}
