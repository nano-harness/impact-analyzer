package diff

import "github.com/nano-harness/impact-analyzer/internal/model"

// IsLineInSignature approximates signature detection by the symbol declaration
// line. Interface symbols treat their whole body as signature: an interface's
// method set is its public contract, so any change inside the interface
// declaration (method signatures, embedded interfaces) is a contract change.
func IsLineInSignature(sym *model.Symbol, line int, _ []byte) bool {
	if sym == nil {
		return false
	}
	if line == sym.Line {
		return true
	}
	if sym.Kind == model.SymbolKindInterface && sym.EndLine > sym.Line {
		return line > sym.Line && line <= sym.EndLine
	}
	return false
}
