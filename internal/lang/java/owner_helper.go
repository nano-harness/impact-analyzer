package java

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// assignJavaOwners assigns Owner field to methods/constructors (#13)
func (a *JavaAdapter) assignJavaOwners(fileSymbols *model.FileSymbols) {
	for _, sym := range fileSymbols.Symbols {
		if sym.Kind != model.SymbolKindMethod {
			continue
		}
		sym.Owner = enclosingClassName(fileSymbols.Symbols, sym.Line)
	}
}

// enclosingClassName returns the name of the innermost class-like symbol
// (class, interface, enum, record, or anonymous class) whose line range
// contains the given line, or "" when nothing encloses it. Nested classes
// resolve to the nearest enclosing class (smallest containing span).
func enclosingClassName(symbols []*model.Symbol, line int) string {
	var owner string
	bestSpan := int(^uint(0) >> 1) // MaxInt

	for _, sym := range symbols {
		if sym.Kind != model.SymbolKindClass && sym.Kind != model.SymbolKindInterface {
			continue
		}
		if line < sym.Line || sym.EndLine == 0 || line > sym.EndLine {
			continue
		}
		if span := sym.EndLine - sym.Line; span < bestSpan {
			bestSpan = span
			owner = sym.Name
		}
	}
	return owner
}

// receiverHint extracts a best-effort owner hint from a method_invocation's
// object node. Only simple shapes produce a hint:
//
//	userService.find()       -> "userService" (plain identifier)
//	this.userService.find()  -> "userService" (last segment of a field-access chain)
//
// Anything else (bare this/super receivers, nested calls like getSvc().find(),
// casts, array access, ...) yields "". The hint is the receiver variable/field
// NAME, not its type — type inference is beyond tree-sitter, so downstream
// matching must treat it as a best-effort hint only.
func receiverHint(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	switch node.Kind() {
	case "identifier":
		return treesitter.GetNodeText(node, content)
	case "field_access":
		obj := node.ChildByFieldName("object")
		field := node.ChildByFieldName("field")
		if obj == nil || field == nil || !isSimpleReceiver(obj) {
			return ""
		}
		return treesitter.GetNodeText(field, content)
	default:
		return ""
	}
}

// isSimpleReceiver reports whether a receiver expression is a plain
// identifier or a field-access chain rooted at an identifier or this.
func isSimpleReceiver(node *sitter.Node) bool {
	for node != nil && node.Kind() == "field_access" {
		node = node.ChildByFieldName("object")
	}
	if node == nil {
		return false
	}
	switch node.Kind() {
	case "identifier", "this":
		return true
	}
	return false
}
