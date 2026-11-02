package javascript

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

//go:embed js_symbols.scm
var jsSymbolsQuery string

//go:embed js_calls.scm
var jsCallsQuery string

func init() {
	lang.Register(&JavaScriptAdapter{})
}

// JavaScriptAdapter implements the LanguageAdapter interface for JavaScript
type JavaScriptAdapter struct{}

func (a *JavaScriptAdapter) Language() model.Language {
	return model.LanguageJavaScript
}

func (a *JavaScriptAdapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	var fileSymbols *model.FileSymbols
	err := treesitter.WithParser(model.LanguageJavaScript, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, jsSymbolsQuery)
		if err != nil {
			return err
		}

		fileSymbols = &model.FileSymbols{
			File:     filePath,
			Language: model.LanguageJavaScript,
			Symbols:  make([]*model.Symbol, 0),
			Imports:  make([]string, 0),
		}

		// Track exported symbols for marking
		exportedNames := make(map[string]bool)

		// First pass: collect all exported names and imports
		for _, match := range matches {
			if importSource, ok := match.Captures["import.source"]; ok {
				source := treesitter.GetNodeText(importSource, content)
				source = strings.Trim(source, "\"'")
				fileSymbols.Imports = append(fileSymbols.Imports, source)
			}

			// Re-export sources (export * from './x', export { a } from './x')
			// create an import edge to the source module
			if exportSource, ok := match.Captures["export.source"]; ok {
				source := treesitter.GetNodeText(exportSource, content)
				source = strings.Trim(source, "\"'")
				fileSymbols.Imports = append(fileSymbols.Imports, source)
			}

			// Track exported function names
			if exportFunc, ok := match.Captures["export.function.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportFunc, content)] = true
			}

			// Track exported class names
			if exportClass, ok := match.Captures["export.class.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportClass, content)] = true
			}

			// Track exported variable names
			if exportVar, ok := match.Captures["export.var.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportVar, content)] = true
			}

			// Track export specifiers (named exports)
			if exportSpec, ok := match.Captures["export.specifier.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportSpec, content)] = true
			}

			// Track export default identifier (export default foo;)
			if exportDefault, ok := match.Captures["export.default.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportDefault, content)] = true
			}

			// Track named default declarations (export default function foo() {})
			if defaultValue, ok := match.Captures["export.default.value"]; ok {
				if nameNode := defaultValue.ChildByFieldName("name"); nameNode != nil {
					exportedNames[treesitter.GetNodeText(nameNode, content)] = true
				}
			}
		}

		// Second pass: create symbols with proper export marking
		for _, match := range matches {
			// Function declarations
			if funcName, ok := match.Captures["function.name"]; ok {
				name := treesitter.GetNodeText(funcName, content)
				endLine := 0
				if funcDef, ok := match.Captures["function.def"]; ok {
					endLine = int(funcDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindFunction,
					File:       filePath,
					Line:       int(funcName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(funcName.StartPosition().Column) + 1,
					Language:   model.LanguageJavaScript,
					IsExported: a.IsExported(name, exportedNames),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Arrow functions / function expressions assigned to variables
			if arrowName, ok := match.Captures["arrow.name"]; ok {
				name := treesitter.GetNodeText(arrowName, content)
				endLine := 0
				if arrowDef, ok := match.Captures["arrow.def"]; ok {
					endLine = int(arrowDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindFunction,
					File:       filePath,
					Line:       int(arrowName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(arrowName.StartPosition().Column) + 1,
					Language:   model.LanguageJavaScript,
					IsExported: a.IsExported(name, exportedNames),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// var-declared functions (var f = function() {} / var f = () => {})
			if varName, ok := match.Captures["var.name"]; ok {
				name := treesitter.GetNodeText(varName, content)
				endLine := 0
				if varDef, ok := match.Captures["var.def"]; ok {
					endLine = int(varDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindFunction,
					File:       filePath,
					Line:       int(varName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(varName.StartPosition().Column) + 1,
					Language:   model.LanguageJavaScript,
					IsExported: a.IsExported(name, exportedNames),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Class declarations
			if className, ok := match.Captures["class.name"]; ok {
				name := treesitter.GetNodeText(className, content)
				endLine := 0
				if classDef, ok := match.Captures["class.def"]; ok {
					endLine = int(classDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindClass,
					File:       filePath,
					Line:       int(className.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(className.StartPosition().Column) + 1,
					Language:   model.LanguageJavaScript,
					IsExported: a.IsExported(name, exportedNames),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Method definitions
			if methodName, ok := match.Captures["method.name"]; ok {
				endLine := 0
				if methodDef, ok := match.Captures["method.def"]; ok {
					endLine = int(methodDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       treesitter.GetNodeText(methodName, content),
					Kind:       model.SymbolKindMethod,
					File:       filePath,
					Line:       int(methodName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(methodName.StartPosition().Column) + 1,
					Language:   model.LanguageJavaScript,
					IsExported: false, // Methods inherit export status from their class
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}
		}

		return nil // Success
	})
	if err != nil {
		return nil, err
	}
	return fileSymbols, nil
}

func (a *JavaScriptAdapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	var calls []*model.CallSite
	err := treesitter.WithParser(model.LanguageJavaScript, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, jsCallsQuery)
		if err != nil {
			return err
		}

		calls = make([]*model.CallSite, 0)
		for _, match := range matches {
			callSite := &model.CallSite{
				File:     filePath,
				Resolved: false, // JavaScript has dynamic nature
			}

			if funcNode, ok := match.Captures["call.function"]; ok {
				callSite.Line = int(funcNode.StartPosition().Row) + 1
				callSite.Column = int(funcNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(funcNode, content),
					Language: model.LanguageJavaScript,
				}
				calls = append(calls, callSite)
			}

			if methodNode, ok := match.Captures["call.method"]; ok {
				callSite.Line = int(methodNode.StartPosition().Row) + 1
				callSite.Column = int(methodNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(methodNode, content),
					Kind:     model.SymbolKindMethod,
					Language: model.LanguageJavaScript,
				}
				// Receiver owner hint: userService.find() -> Owner "userService".
				// Best-effort (variable/property name, not a type); see receiverHint.
				if objNode, ok := match.Captures["call.object"]; ok {
					callSite.Callee.Owner = receiverHint(objNode, content)
				}
				calls = append(calls, callSite)
			}

			// JSX component usage: <UserCard/> references the component symbol.
			// Only capitalized names (components); lowercase intrinsics like
			// <div/> are skipped to avoid noise.
			if jsxNode, ok := match.Captures["jsx.name"]; ok {
				name, owner := jsxComponentName(jsxNode, content)
				first := name
				if owner != "" {
					first = owner
				}
				if isCapitalized(first) {
					callSite.Line = int(jsxNode.StartPosition().Row) + 1
					callSite.Column = int(jsxNode.StartPosition().Column) + 1
					callSite.EdgeKind = model.EdgeKindTypeRef
					callSite.Callee = &model.Symbol{
						Name:     name,
						Owner:    owner,
						Language: model.LanguageJavaScript,
					}
					calls = append(calls, callSite)
				}
			}
		}

		return nil // Success
	})
	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (a *JavaScriptAdapter) IsTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	dir := filepath.Dir(filePath)
	if strings.Contains(dir, "__tests__") {
		return true
	}
	for _, ext := range []string{".js", ".jsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(base, ".test"+ext) || strings.HasSuffix(base, ".spec"+ext) {
			return true
		}
	}
	return false
}

func (a *JavaScriptAdapter) IsExported(symbolName string, context interface{}) bool {
	// Context carries the exported-name set collected from the file's export
	// statements (export function/class/const, export { name }, ...)
	if exportedNames, ok := context.(map[string]bool); ok {
		return exportedNames[symbolName]
	}
	return false
}

func (a *JavaScriptAdapter) ResolveImports(filePath string) ([]string, error) {
	fileSymbols, err := a.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file for imports: %w", err)
	}
	return fileSymbols.Imports, nil
}

// isTestSymbolName checks if a symbol name looks like a test function
func (a *JavaScriptAdapter) isTestSymbolName(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.Contains(lowerName, "test") ||
		strings.Contains(lowerName, "should") ||
		strings.HasPrefix(lowerName, "it") ||
		strings.HasPrefix(lowerName, "describe")
}
