package rust

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func init() { lang.Register(&RustAdapter{}) }

type RustAdapter struct{}

func (a *RustAdapter) Language() model.Language { return model.LanguageRust }

func (a *RustAdapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	var fileSymbols *model.FileSymbols
	err := treesitter.WithParser(model.LanguageRust, func(parser *treesitter.Parser) error {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		tree, err := parser.ParseContent(content)
		if err != nil {
			return err
		}
		defer tree.Close()

		fileSymbols = &model.FileSymbols{File: filePath, Language: model.LanguageRust, Package: rustModulePath(filePath), Symbols: []*model.Symbol{}, Imports: []string{}}
		isTest := a.IsTestFile(filePath) || strings.Contains(string(content), "#[cfg(test)]")
		walk(tree.RootNode(), func(node *sitter.Node) {
			switch node.Kind() {
			case "use_declaration":
				fileSymbols.Imports = append(fileSymbols.Imports, rustUsePaths(node, content, filePath)...)
			case "function_item":
				name := node.ChildByFieldName("name")
				if name == nil {
					return
				}
				kind := model.SymbolKindFunction
				owner := ""
				if parent := node.Parent(); parent != nil && parent.Kind() == "declaration_list" && parent.Parent() != nil && parent.Parent().Kind() == "impl_item" {
					kind = model.SymbolKindMethod
					if typ := parent.Parent().ChildByFieldName("type"); typ != nil {
						owner = treesitter.GetNodeText(typ, content)
					}
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, &model.Symbol{Name: treesitter.GetNodeText(name, content), Kind: kind, Owner: owner, File: filePath, Line: int(name.StartPosition().Row) + 1, EndLine: int(node.EndPosition().Row) + 1, Column: int(name.StartPosition().Column) + 1, Language: model.LanguageRust, Package: fileSymbols.Package, IsExported: a.IsExported("", node), IsTest: isTest})
			case "struct_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindStruct, filePath, content, a, isTest)
			case "enum_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindStruct, filePath, content, a, isTest)
				addEnumVariants(fileSymbols, node, filePath, content, a, isTest)
			case "type_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindStruct, filePath, content, a, isTest)
			case "const_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindConstant, filePath, content, a, isTest)
			case "static_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindConstant, filePath, content, a, isTest)
			case "trait_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindInterface, filePath, content, a, isTest)
			case "mod_item":
				addTypeSymbol(fileSymbols, node, "name", model.SymbolKindModule, filePath, content, a, isTest)
			}
		})

		return nil // Success
	})
	if err != nil {
		return nil, err
	}
	return fileSymbols, nil
}

func addTypeSymbol(fs *model.FileSymbols, node *sitter.Node, field string, kind model.SymbolKind, filePath string, content []byte, a *RustAdapter, isTest bool) {
	name := node.ChildByFieldName(field)
	if name == nil {
		return
	}
	fs.Symbols = append(fs.Symbols, &model.Symbol{Name: treesitter.GetNodeText(name, content), Kind: kind, File: filePath, Line: int(name.StartPosition().Row) + 1, EndLine: int(node.EndPosition().Row) + 1, Column: int(name.StartPosition().Column) + 1, Language: model.LanguageRust, Package: fs.Package, IsExported: a.IsExported("", node), IsTest: isTest})
}

func addEnumVariants(fs *model.FileSymbols, enumNode *sitter.Node, filePath string, content []byte, a *RustAdapter, isTest bool) {
	enumName := enumNode.ChildByFieldName("name")
	if enumName == nil {
		return
	}
	owner := treesitter.GetNodeText(enumName, content)
	exported := a.IsExported("", enumNode)
	body := enumNode.ChildByFieldName("body")
	if body == nil {
		return
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		variant := body.NamedChild(i)
		if variant.Kind() != "enum_variant" {
			continue
		}
		name := variant.ChildByFieldName("name")
		if name == nil {
			continue
		}
		fs.Symbols = append(fs.Symbols, &model.Symbol{Name: treesitter.GetNodeText(name, content), Kind: model.SymbolKindConstant, Owner: owner, File: filePath, Line: int(name.StartPosition().Row) + 1, EndLine: int(variant.EndPosition().Row) + 1, Column: int(name.StartPosition().Column) + 1, Language: model.LanguageRust, Package: fs.Package, IsExported: exported, IsTest: isTest})
	}
}

func (a *RustAdapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	var calls []*model.CallSite
	err := treesitter.WithParser(model.LanguageRust, func(parser *treesitter.Parser) error {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		tree, err := parser.ParseContent(content)
		if err != nil {
			return err
		}
		defer tree.Close()

		calls = make([]*model.CallSite, 0)
		walk(tree.RootNode(), func(node *sitter.Node) {
			switch node.Kind() {
			case "impl_item":
				// impl Trait for Type -> inheritance edge Type -> Trait (#22)
				trait := node.ChildByFieldName("trait")
				if trait == nil {
					return
				}
				callSite := &model.CallSite{
					File:     filePath,
					Line:     int(trait.StartPosition().Row) + 1,
					Column:   int(trait.StartPosition().Column) + 1,
					Resolved: true,
					EdgeKind: model.EdgeKindInheritance,
					Callee: &model.Symbol{
						Name:     treesitter.GetNodeText(trait, content),
						Kind:     model.SymbolKindInterface,
						Language: model.LanguageRust,
					},
				}
				if typ := node.ChildByFieldName("type"); typ != nil {
					callSite.Caller = &model.Symbol{
						Name:     treesitter.GetNodeText(typ, content),
						Kind:     model.SymbolKindStruct,
						Language: model.LanguageRust,
					}
				}
				calls = append(calls, callSite)
			case "call_expression":
				fn := node.ChildByFieldName("function")
				name, qualifier := callName(fn, content)
				if name == "" {
					return
				}
				callee := &model.Symbol{Name: name, Language: model.LanguageRust}
				// A qualified path (greeter::greet, crate::greeter::greet, ...)
				// resolves to a module path, giving the edge an exact package.
				if qualifier != "" {
					callee.Package = resolveRustPath(filePath, qualifier)
				}
				calls = append(calls, &model.CallSite{File: filePath, Line: int(fn.StartPosition().Row) + 1, Column: int(fn.StartPosition().Column) + 1, Resolved: true, Callee: callee})
			case "token_tree":
				// Macro bodies (format!, println!, ...) parse as raw token
				// trees; recover the calls they contain (ident followed by a
				// paren group), including path and receiver forms.
				calls = append(calls, macroCalls(node, filePath, content)...)
			}
		})

		return nil // Success
	})
	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (a *RustAdapter) IsTestFile(filePath string) bool {
	clean := filepath.ToSlash(filePath)
	return strings.Contains(clean, "/tests/") || strings.HasSuffix(clean, "_test.rs")
}

// rustModulePath derives a crate module path from the file's location (Rust
// has no package declaration): the directory stands in for the module of
// lib.rs/main.rs/mod.rs; any other file forms its own submodule named after
// the file stem (src/foo.rs -> <dir>/foo).
func rustModulePath(filePath string) string {
	dir := filepath.ToSlash(filepath.Dir(filePath))
	base := filepath.Base(filePath)
	switch base {
	case "lib.rs", "main.rs", "mod.rs":
		return dir
	}
	return dir + "/" + strings.TrimSuffix(base, filepath.Ext(base))
}

// rustCrateRootDir locates the crate root directory (the one whose modules
// `crate::` refers to): the src dir under the nearest ancestor holding a
// Cargo.toml, else the nearest ancestor literally named "src", else the
// file's own directory.
func rustCrateRootDir(filePath string) string {
	dir := filepath.Dir(filePath)
	for d := dir; ; d = filepath.Dir(d) {
		if st, err := os.Stat(filepath.Join(d, "Cargo.toml")); err == nil && !st.IsDir() {
			src := filepath.Join(d, "src")
			if st, err := os.Stat(src); err == nil && st.IsDir() {
				return filepath.ToSlash(src)
			}
			return filepath.ToSlash(d)
		}
		if parent := filepath.Dir(d); parent == d {
			break
		}
	}
	for d := dir; ; d = filepath.Dir(d) {
		if filepath.Base(d) == "src" {
			return filepath.ToSlash(d)
		}
		if parent := filepath.Dir(d); parent == d {
			break
		}
	}
	return filepath.ToSlash(dir)
}

// resolveRustPath resolves a :: path qualifier (the path before the final
// segment of a use/call path) to a slash module path. Leading crate/self/
// super keywords are honored; anything else resolves relative to the file's
// own module (Rust 2018 in-crate paths).
func resolveRustPath(filePath, qualifier string) string {
	segs := strings.Split(qualifier, "::")
	base := rustModulePath(filePath)
	switch segs[0] {
	case "crate":
		base = rustCrateRootDir(filePath)
		segs = segs[1:]
	case "self":
		segs = segs[1:]
	default:
		for len(segs) > 0 && segs[0] == "super" {
			base = path.Dir(base)
			segs = segs[1:]
		}
	}
	return strings.Join(append([]string{base}, segs...), "/")
}

// rustUsePaths converts a use declaration into the module paths it evidences:
// the raw :: path plus its resolved slash form and resolved parent module (a
// use path usually names an item inside the module, and callers match on the
// module path). External-crate paths stay in their raw :: form.
func rustUsePaths(node *sitter.Node, content []byte, filePath string) []string {
	var paths []string
	seen := make(map[string]bool)
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, raw := range usePathTexts(node.ChildByFieldName("argument"), content, "") {
		add(raw)
		resolved := resolveRustPath(filePath, raw)
		add(resolved)
		if parent := path.Dir(resolved); parent != resolved && parent != "." {
			add(parent)
		}
	}
	return paths
}

// usePathTexts flattens a use argument into full :: paths, expanding nested
// use lists (use crate::{a, b::c}) and stripping aliases/wildcards.
func usePathTexts(node *sitter.Node, content []byte, prefix string) []string {
	if node == nil {
		return nil
	}
	join := func(text string) string {
		if prefix != "" {
			return prefix + "::" + text
		}
		return text
	}
	switch node.Kind() {
	case "identifier", "scoped_identifier":
		return []string{join(treesitter.GetNodeText(node, content))}
	case "use_as_clause":
		if p := node.ChildByFieldName("path"); p != nil {
			return usePathTexts(p, content, prefix)
		}
	case "use_wildcard":
		for i := uint(0); i < node.NamedChildCount(); i++ {
			if child := node.NamedChild(i); child.Kind() == "identifier" || child.Kind() == "scoped_identifier" {
				return usePathTexts(child, content, prefix)
			}
		}
	case "scoped_use_list":
		base := prefix
		if p := node.ChildByFieldName("path"); p != nil {
			base = join(treesitter.GetNodeText(p, content))
		}
		var out []string
		for i := uint(0); i < node.NamedChildCount(); i++ {
			list := node.NamedChild(i)
			if list.Kind() != "use_list" {
				continue
			}
			for j := uint(0); j < list.NamedChildCount(); j++ {
				out = append(out, usePathTexts(list.NamedChild(j), content, base)...)
			}
		}
		return out
	}
	return nil
}

func (a *RustAdapter) IsExported(symbolName string, context interface{}) bool {
	if node, ok := context.(*sitter.Node); ok {
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child.Kind() == "visibility_modifier" {
				return true
			}
		}
	}
	return false
}

func (a *RustAdapter) ResolveImports(filePath string) ([]string, error) {
	fs, err := a.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file for imports: %w", err)
	}
	return fs.Imports, nil
}

func walk(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := uint(0); i < node.NamedChildCount(); i++ {
		walk(node.NamedChild(i), fn)
	}
}

// callName extracts the callee's bare name and, for qualified paths, the
// qualifying module path (e.g. "crate::greeter" for crate::greeter::greet).
func callName(fn *sitter.Node, content []byte) (name, qualifier string) {
	if fn == nil {
		return "", ""
	}
	switch fn.Kind() {
	case "identifier":
		return treesitter.GetNodeText(fn, content), ""
	case "scoped_identifier":
		text := treesitter.GetNodeText(fn, content)
		if idx := strings.LastIndex(text, "::"); idx >= 0 {
			return text[idx+2:], text[:idx]
		}
		return text, ""
	case "field_expression":
		field := fn.ChildByFieldName("field")
		return treesitter.GetNodeText(field, content), ""
	default:
		return "", ""
	}
}

// macroCalls recovers call sites from a macro body's token tree: an
// identifier immediately followed by a paren token tree is a call, with a
// leading "::" chain as qualifier and a leading "." marking a receiver call.
// Only direct children are scanned; the walk recurses into nested token
// trees, so calls at any macro nesting depth are found exactly once.
func macroCalls(tt *sitter.Node, filePath string, content []byte) []*model.CallSite {
	var calls []*model.CallSite
	for i := uint(0); i+2 <= tt.ChildCount(); i++ {
		ident := tt.Child(i)
		if ident == nil || ident.Kind() != "identifier" {
			continue
		}
		args := tt.Child(i + 1)
		if args == nil || args.Kind() != "token_tree" || args.ChildCount() == 0 || args.Child(0).Kind() != "(" {
			continue
		}
		name := treesitter.GetNodeText(ident, content)
		// Extend a leading path qualifier (crate::greeter::greet) out of the
		// "::"-separated segments preceding the name; a "." receiver stops it.
		qualifier := ""
		for j := i; j >= 2 && tt.Child(j-1).Kind() == "::"; j -= 2 {
			seg := tt.Child(j - 2)
			if kind := seg.Kind(); kind != "identifier" && kind != "crate" && kind != "self" && kind != "super" {
				break
			}
			qualifier = treesitter.GetNodeText(seg, content) + "::" + qualifier
		}
		qualifier = strings.TrimSuffix(qualifier, "::")
		callee := &model.Symbol{Name: name, Language: model.LanguageRust}
		if qualifier != "" {
			callee.Package = resolveRustPath(filePath, qualifier)
		}
		calls = append(calls, &model.CallSite{File: filePath, Line: int(ident.StartPosition().Row) + 1, Column: int(ident.StartPosition().Column) + 1, Resolved: true, Callee: callee})
	}
	return calls
}
