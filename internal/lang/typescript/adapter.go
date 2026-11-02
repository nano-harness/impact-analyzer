package typescript

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

//go:embed ts_symbols.scm
var tsSymbolsQuery string

//go:embed ts_calls.scm
var tsCallsQuery string

// tsxJSXQuery captures JSX component usage. JSX nodes only exist in the TSX
// grammar, so this query is run separately and only for .tsx files (running it
// against the plain TS grammar would fail to compile).
const tsxJSXQuery = `
(jsx_opening_element
  name: [(identifier) (member_expression)] @jsx.name)

(jsx_self_closing_element
  name: [(identifier) (member_expression)] @jsx.name)
`

func init() {
	lang.Register(&TypeScriptAdapter{})
}

// TypeScriptAdapter implements the LanguageAdapter interface for TypeScript
type TypeScriptAdapter struct{}

func (a *TypeScriptAdapter) Language() model.Language {
	return model.LanguageTypeScript
}

func (a *TypeScriptAdapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	var fileSymbols *model.FileSymbols
	err := treesitter.WithParser(model.LanguageTypeScript, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, tsSymbolsQuery)
		if err != nil {
			return err
		}

		fileSymbols = &model.FileSymbols{
			File:     filePath,
			Language: model.LanguageTypeScript,
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

			// Track exported class names (includes abstract classes)
			if exportClass, ok := match.Captures["export.class.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportClass, content)] = true
			}

			// Track exported interface names
			if exportInterface, ok := match.Captures["export.interface.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportInterface, content)] = true
			}

			// Track exported type alias names
			if exportType, ok := match.Captures["export.type.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportType, content)] = true
			}

			// Track exported enum names
			if exportEnum, ok := match.Captures["export.enum.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportEnum, content)] = true
			}

			// Track exported namespace/module names
			if exportNs, ok := match.Captures["export.namespace.name"]; ok {
				exportedNames[treesitter.GetNodeText(exportNs, content)] = true
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
					Name:           name,
					Kind:           model.SymbolKindFunction,
					File:           filePath,
					Line:           int(funcName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(funcName.StartPosition().Column) + 1,
					Language:       model.LanguageTypeScript,
					TypeParameters: a.extractTypeParameters(match, "function.type_params", content),
					IsExported:     exportedNames[name],
					IsTest:         a.IsTestFile(filePath),
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
					Language:   model.LanguageTypeScript,
					IsExported: exportedNames[name],
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
					Language:   model.LanguageTypeScript,
					IsExported: exportedNames[name],
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Interface declarations
			if interfaceName, ok := match.Captures["interface.name"]; ok {
				name := treesitter.GetNodeText(interfaceName, content)
				endLine := 0
				if interfaceDef, ok := match.Captures["interface.def"]; ok {
					endLine = int(interfaceDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindInterface,
					File:           filePath,
					Line:           int(interfaceName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(interfaceName.StartPosition().Column) + 1,
					Language:       model.LanguageTypeScript,
					TypeParameters: a.extractTypeParameters(match, "interface.type_params", content),
					IsExported:     exportedNames[name],
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Class declarations (includes abstract classes)
			if className, ok := match.Captures["class.name"]; ok {
				name := treesitter.GetNodeText(className, content)
				endLine := 0
				if classDef, ok := match.Captures["class.def"]; ok {
					endLine = int(classDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindClass,
					File:           filePath,
					Line:           int(className.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(className.StartPosition().Column) + 1,
					Language:       model.LanguageTypeScript,
					TypeParameters: a.extractTypeParameters(match, "class.type_params", content),
					IsExported:     exportedNames[name],
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Method definitions (in classes)
			if methodName, ok := match.Captures["method.name"]; ok {
				endLine := 0
				if methodDef, ok := match.Captures["method.def"]; ok {
					endLine = int(methodDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       treesitter.GetNodeText(methodName, content),
					Kind:       model.SymbolKindMethod,
					Owner:      ownerName(methodName, content),
					File:       filePath,
					Line:       int(methodName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(methodName.StartPosition().Column) + 1,
					Language:   model.LanguageTypeScript,
					IsExported: false, // Methods inherit export status from their class
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Method signatures (in interfaces and abstract classes)
			if sigName, ok := match.Captures["method_sig.name"]; ok {
				endLine := 0
				if sigDef, ok := match.Captures["method_sig.def"]; ok {
					endLine = int(sigDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       treesitter.GetNodeText(sigName, content),
					Kind:       model.SymbolKindMethod,
					Owner:      ownerName(sigName, content),
					File:       filePath,
					Line:       int(sigName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(sigName.StartPosition().Column) + 1,
					Language:   model.LanguageTypeScript,
					IsExported: false, // Methods inherit export status from their interface/class
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Type alias declarations (treated as named types)
			if typeAliasName, ok := match.Captures["type_alias.name"]; ok {
				name := treesitter.GetNodeText(typeAliasName, content)
				endLine := 0
				if typeAliasDef, ok := match.Captures["type_alias.def"]; ok {
					endLine = int(typeAliasDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindInterface,
					File:           filePath,
					Line:           int(typeAliasName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(typeAliasName.StartPosition().Column) + 1,
					Language:       model.LanguageTypeScript,
					TypeParameters: a.extractTypeParameters(match, "type_alias.type_params", content),
					IsExported:     exportedNames[name],
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Enum declarations (treated like classes, as in the Java adapter)
			if enumName, ok := match.Captures["enum.name"]; ok {
				name := treesitter.GetNodeText(enumName, content)
				endLine := 0
				if enumDef, ok := match.Captures["enum.def"]; ok {
					endLine = int(enumDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindClass,
					File:       filePath,
					Line:       int(enumName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(enumName.StartPosition().Column) + 1,
					Language:   model.LanguageTypeScript,
					IsExported: exportedNames[name],
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Namespaces / internal modules
			if nsName, ok := match.Captures["namespace.name"]; ok {
				name := treesitter.GetNodeText(nsName, content)
				endLine := 0
				if nsDef, ok := match.Captures["namespace.def"]; ok {
					endLine = int(nsDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindModule,
					File:       filePath,
					Line:       int(nsName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(nsName.StartPosition().Column) + 1,
					Language:   model.LanguageTypeScript,
					IsExported: exportedNames[name],
					IsTest:     a.IsTestFile(filePath),
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

func (a *TypeScriptAdapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	var calls []*model.CallSite
	err := treesitter.WithParser(model.LanguageTypeScript, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, tsCallsQuery)
		if err != nil {
			return err
		}

		calls = make([]*model.CallSite, 0)
		for _, match := range matches {
			callSite := &model.CallSite{
				File:     filePath,
				Resolved: true, // TypeScript has better static analysis than JS
			}

			if funcNode, ok := match.Captures["call.function"]; ok {
				callSite.Line = int(funcNode.StartPosition().Row) + 1
				callSite.Column = int(funcNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(funcNode, content),
					Language: model.LanguageTypeScript,
				}
				calls = append(calls, callSite)
			}

			if methodNode, ok := match.Captures["call.method"]; ok {
				callSite.Line = int(methodNode.StartPosition().Row) + 1
				callSite.Column = int(methodNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(methodNode, content),
					Kind:     model.SymbolKindMethod,
					Language: model.LanguageTypeScript,
				}
				// Receiver owner hint: userService.find() -> Owner "userService".
				// Best-effort (variable/property name, not a type); see receiverHint.
				if objNode, ok := match.Captures["call.object"]; ok {
					callSite.Callee.Owner = receiverHint(objNode, content)
				}
				calls = append(calls, callSite)
			}
		}

		// JSX component usage: <UserCard/> references the component symbol.
		// JSX nodes only exist in the TSX grammar, so this runs only for .tsx
		// files. Only capitalized names (components); lowercase intrinsics like
		// <div/> are skipped to avoid noise.
		if strings.HasSuffix(filePath, ".tsx") {
			jsxMatches, err := parser.Query(tree, tsxJSXQuery)
			if err != nil {
				return err
			}
			for _, match := range jsxMatches {
				jsxNode, ok := match.Captures["jsx.name"]
				if !ok {
					continue
				}
				name, owner := jsxComponentName(jsxNode, content)
				first := name
				if owner != "" {
					first = owner
				}
				if !isCapitalized(first) {
					continue
				}
				calls = append(calls, &model.CallSite{
					File:     filePath,
					Line:     int(jsxNode.StartPosition().Row) + 1,
					Column:   int(jsxNode.StartPosition().Column) + 1,
					EdgeKind: model.EdgeKindTypeRef,
					Resolved: true,
					Callee: &model.Symbol{
						Name:     name,
						Owner:    owner,
						Language: model.LanguageTypeScript,
					},
				})
			}
		}

		return nil // Success
	})
	if err != nil {
		return nil, err
	}
	return calls, nil
}

func (a *TypeScriptAdapter) IsTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	dir := filepath.Dir(filePath)
	if strings.Contains(dir, "__tests__") {
		return true
	}
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
		if strings.HasSuffix(base, ".test"+ext) || strings.HasSuffix(base, ".spec"+ext) {
			return true
		}
	}
	return false
}

func (a *TypeScriptAdapter) IsExported(symbolName string, context interface{}) bool {
	// Would need AST context to determine if symbol is exported
	return false
}

func (a *TypeScriptAdapter) ResolveImports(filePath string) ([]string, error) {
	fileSymbols, err := a.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file for imports: %w", err)
	}
	return fileSymbols.Imports, nil
}

// isTestSymbolName checks if a symbol name looks like a test function
func (a *TypeScriptAdapter) isTestSymbolName(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.Contains(lowerName, "test") ||
		strings.Contains(lowerName, "should") ||
		strings.HasPrefix(lowerName, "it") ||
		strings.HasPrefix(lowerName, "describe")
}

// extractTypeParameters extracts type parameter names from a TreeSitter match
// For TypeScript: type_parameters contains type_parameter nodes like <T, U extends Base>
func (a *TypeScriptAdapter) extractTypeParameters(match *treesitter.QueryMatch, captureKey string, content []byte) []string {
	node, ok := match.Captures[captureKey]
	if !ok {
		return nil
	}

	text := treesitter.GetNodeText(node, content)
	if text == "" {
		return nil
	}

	// Parse type parameters from text like "<T>" or "<T, U>" or "<T extends Base, U>"
	// Remove angle brackets
	text = strings.TrimPrefix(text, "<")
	text = strings.TrimSuffix(text, ">")

	var typeParams []string
	parts := strings.Split(text, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Extract just the type parameter name (before extends/=)
		if idx := strings.Index(part, " extends "); idx != -1 {
			part = part[:idx]
		}
		if idx := strings.Index(part, "="); idx != -1 {
			part = part[:idx]
		}
		part = strings.TrimSpace(part)
		if part != "" {
			typeParams = append(typeParams, part)
		}
	}

	return typeParams
}
