package javascript

import (
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
)

// receiverHint extracts a best-effort owner hint from a member call's object
// node. Only simple shapes produce a hint:
//
//	userService.find()       -> "userService" (plain identifier)
//	this.userService.find()  -> "userService" (last segment of a member chain)
//
// Anything else (bare this/super receivers, nested calls like getSvc().find(),
// subscripts, ...) yields "". The hint is the receiver variable/property NAME,
// not its type — type inference is beyond tree-sitter, so downstream matching
// must treat it as a best-effort hint only.
func receiverHint(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	switch node.Kind() {
	case "identifier":
		return treesitter.GetNodeText(node, content)
	case "member_expression":
		obj := node.ChildByFieldName("object")
		prop := node.ChildByFieldName("property")
		if obj == nil || prop == nil || !isSimpleReceiver(obj) {
			return ""
		}
		return treesitter.GetNodeText(prop, content)
	default:
		return ""
	}
}

// isSimpleReceiver reports whether a receiver expression is a plain
// identifier or a member-expression chain rooted at an identifier or this.
func isSimpleReceiver(node *sitter.Node) bool {
	for node != nil && node.Kind() == "member_expression" {
		node = node.ChildByFieldName("object")
	}
	if node == nil {
		return false
	}
	return node.Kind() == "identifier" || node.Kind() == "this"
}

// jsxComponentName extracts the referenced component from a JSX element name
// node. Plain identifiers yield the name itself (<UserCard/> -> "UserCard");
// member expressions yield the last segment with the object as owner hint
// (<Foo.Bar/> -> "Bar", owner "Foo"). Anything else yields "".
func jsxComponentName(node *sitter.Node, content []byte) (name, owner string) {
	if node == nil {
		return "", ""
	}
	switch node.Kind() {
	case "identifier":
		return treesitter.GetNodeText(node, content), ""
	case "member_expression":
		obj := node.ChildByFieldName("object")
		prop := node.ChildByFieldName("property")
		if prop == nil {
			return "", ""
		}
		if obj != nil {
			owner = treesitter.GetNodeText(obj, content)
		}
		return treesitter.GetNodeText(prop, content), owner
	default:
		return "", ""
	}
}

// isCapitalized reports whether the first rune of s is uppercase, which
// distinguishes JSX components (<UserCard/>) from intrinsic elements (<div/>).
func isCapitalized(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}
