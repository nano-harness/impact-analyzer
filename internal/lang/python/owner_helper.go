package python

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// assignPythonOwners assigns Owner field to methods (#14)
func assignPythonOwners(fileSymbols *model.FileSymbols, lines []string) {
	// Build class stack while iterating through lines
	type classScope struct {
		name   string
		indent int
	}

	// Map line numbers to classes
	lineToClass := make(map[int]string)
	var classStack []classScope

	for i, line := range lines {
		lineNo := i + 1
		curIndent := indent(line)

		// Pop classes whose indent >= curIndent
		for len(classStack) > 0 && classStack[len(classStack)-1].indent >= curIndent {
			classStack = classStack[:len(classStack)-1]
		}

		// Check if this line starts a class
		if strings.Contains(line, "class ") {
			classStack = append(classStack, classScope{
				name:   extractClassName(line),
				indent: curIndent,
			})
		}

		// Record current class for this line
		if len(classStack) > 0 {
			lineToClass[lineNo] = classStack[len(classStack)-1].name
		}
	}

	// Assign owners to methods
	for _, sym := range fileSymbols.Symbols {
		if sym.Kind == model.SymbolKindFunction || sym.Kind == model.SymbolKindMethod {
			if owner, ok := lineToClass[sym.Line]; ok {
				sym.Owner = owner
			}
		}
	}
}

// receiverHint extracts a best-effort owner hint from a method call's object
// node. Only simple shapes produce a hint:
//
//	userService.find()  -> "userService" (plain identifier)
//	self.repo.find()    -> "repo" (last segment of an attribute chain)
//
// Bare self/cls receivers yield "" — they resolve to the enclosing class,
// which the caller's own Owner already covers; a literal "self"/"cls" hint
// would only contradict definition-side class names downstream. Anything
// complex (nested calls like get_svc().find(), subscripts, ...) also yields
// "". The hint is the receiver variable/attribute NAME, not its type — type
// inference is beyond tree-sitter, so downstream matching must treat it as a
// best-effort hint only.
func receiverHint(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	switch node.Kind() {
	case "identifier":
		name := treesitter.GetNodeText(node, content)
		if name == "self" || name == "cls" {
			return ""
		}
		return name
	case "attribute":
		obj := node.ChildByFieldName("object")
		attr := node.ChildByFieldName("attribute")
		if obj == nil || attr == nil || !isSimpleReceiver(obj) {
			return ""
		}
		return treesitter.GetNodeText(attr, content)
	default:
		return ""
	}
}

// isSimpleReceiver reports whether a receiver expression is a plain
// identifier or an attribute chain rooted at an identifier (e.g. self.repo).
func isSimpleReceiver(node *sitter.Node) bool {
	for node != nil && node.Kind() == "attribute" {
		node = node.ChildByFieldName("object")
	}
	return node != nil && node.Kind() == "identifier"
}

func extractClassName(line string) string {
	// Extract class name from "class Name:" or "class Name(Base):"
	parts := strings.Fields(line)
	for i, part := range parts {
		if part == "class" && i+1 < len(parts) {
			name := parts[i+1]
			// Remove trailing : or (
			name = strings.TrimSuffix(name, ":")
			if idx := strings.Index(name, "("); idx != -1 {
				name = name[:idx]
			}
			return name
		}
	}
	return ""
}
