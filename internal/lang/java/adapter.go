package java

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

//go:embed java_symbols.scm
var javaSymbolsQuery string

//go:embed java_calls.scm
var javaCallsQuery string

func init() {
	lang.Register(&JavaAdapter{})
}

// JavaAdapter implements the LanguageAdapter interface for Java
type JavaAdapter struct{}

func (a *JavaAdapter) Language() model.Language {
	return model.LanguageJava
}

func (a *JavaAdapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	var fileSymbols *model.FileSymbols
	err := treesitter.WithParser(model.LanguageJava, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, javaSymbolsQuery)
		if err != nil {
			return err
		}

		fileSymbols = &model.FileSymbols{
			File:     filePath,
			Language: model.LanguageJava,
			Symbols:  make([]*model.Symbol, 0),
			Imports:  make([]string, 0),
		}

		for _, match := range matches {
			if pkgNode, ok := match.Captures["package.name"]; ok {
				fileSymbols.Package = treesitter.GetNodeText(pkgNode, content)
			}

			if importPath, ok := match.Captures["import.path"]; ok {
				path := treesitter.GetNodeText(importPath, content)
				fileSymbols.Imports = append(fileSymbols.Imports, path)
			}

			// Class declarations
			if className, ok := match.Captures["class.name"]; ok {
				endLine := 0
				if classDef, ok := match.Captures["class.def"]; ok {
					endLine = int(classDef.EndPosition().Row) + 1
				}
				modifiers := ""
				if modNode, ok := match.Captures["class.modifiers"]; ok {
					modifiers = treesitter.GetNodeText(modNode, content)
				}
				name := treesitter.GetNodeText(className, content)
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindClass,
					File:           filePath,
					Line:           int(className.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(className.StartPosition().Column) + 1,
					Language:       model.LanguageJava,
					Package:        fileSymbols.Package,
					TypeParameters: a.extractTypeParameters(match, "class.type_params", content),
					IsExported:     a.IsExported(name, modifiers), // #8: use class modifiers
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Interface declarations
			if interfaceName, ok := match.Captures["interface.name"]; ok {
				endLine := 0
				if interfaceDef, ok := match.Captures["interface.def"]; ok {
					endLine = int(interfaceDef.EndPosition().Row) + 1
				}
				modifiers := ""
				if modNode, ok := match.Captures["interface.modifiers"]; ok {
					modifiers = treesitter.GetNodeText(modNode, content)
				}
				name := treesitter.GetNodeText(interfaceName, content)
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindInterface,
					File:           filePath,
					Line:           int(interfaceName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(interfaceName.StartPosition().Column) + 1,
					Language:       model.LanguageJava,
					Package:        fileSymbols.Package,
					TypeParameters: a.extractTypeParameters(match, "interface.type_params", content),
					IsExported:     a.IsExported(name, modifiers), // #8: use interface modifiers
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Enum declarations (treated like classes)
			if enumName, ok := match.Captures["enum.name"]; ok {
				endLine := 0
				if enumDef, ok := match.Captures["enum.def"]; ok {
					endLine = int(enumDef.EndPosition().Row) + 1
				}
				modifiers := ""
				if modNode, ok := match.Captures["enum.modifiers"]; ok {
					modifiers = treesitter.GetNodeText(modNode, content)
				}
				name := treesitter.GetNodeText(enumName, content)
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindClass,
					File:       filePath,
					Line:       int(enumName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(enumName.StartPosition().Column) + 1,
					Language:   model.LanguageJava,
					Package:    fileSymbols.Package,
					IsExported: a.IsExported(name, modifiers),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Record declarations (treated like classes)
			if recordName, ok := match.Captures["record.name"]; ok {
				endLine := 0
				if recordDef, ok := match.Captures["record.def"]; ok {
					endLine = int(recordDef.EndPosition().Row) + 1
				}
				modifiers := ""
				if modNode, ok := match.Captures["record.modifiers"]; ok {
					modifiers = treesitter.GetNodeText(modNode, content)
				}
				name := treesitter.GetNodeText(recordName, content)
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindClass,
					File:           filePath,
					Line:           int(recordName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(recordName.StartPosition().Column) + 1,
					Language:       model.LanguageJava,
					Package:        fileSymbols.Package,
					TypeParameters: a.extractTypeParameters(match, "record.type_params", content),
					IsExported:     a.IsExported(name, modifiers),
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Method declarations
			if methodName, ok := match.Captures["method.name"]; ok {
				modifiers := ""
				if modNode, ok := match.Captures["method.modifiers"]; ok {
					modifiers = treesitter.GetNodeText(modNode, content)
				}
				endLine := 0
				if methodDef, ok := match.Captures["method.def"]; ok {
					endLine = int(methodDef.EndPosition().Row) + 1
				}
				name := treesitter.GetNodeText(methodName, content)
				symbol := &model.Symbol{
					Name:           name,
					Kind:           model.SymbolKindMethod,
					File:           filePath,
					Line:           int(methodName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(methodName.StartPosition().Column) + 1,
					Language:       model.LanguageJava,
					Package:        fileSymbols.Package,
					TypeParameters: a.extractTypeParameters(match, "method.type_params", content),
					IsExported:     a.IsExported(name, modifiers),
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Constructor declarations (#6)
			if ctorName, ok := match.Captures["constructor.name"]; ok {
				modifiers := ""
				if modNode, ok := match.Captures["constructor.modifiers"]; ok {
					modifiers = treesitter.GetNodeText(modNode, content)
				}
				endLine := 0
				if ctorDef, ok := match.Captures["constructor.def"]; ok {
					endLine = int(ctorDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       "<init>",
					Kind:       model.SymbolKindMethod,
					File:       filePath,
					Line:       int(ctorName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(ctorName.StartPosition().Column) + 1,
					Language:   model.LanguageJava,
					Package:    fileSymbols.Package,
					IsExported: a.IsExported("<init>", modifiers),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// #23: Anonymous class declarations
			if anonClassType, ok := match.Captures["anon_class.type"]; ok {
				// Anonymous classes get synthetic names like "Interface$1"
				// We use the line number to make them unique
				typeName := treesitter.GetNodeText(anonClassType, content)
				lineNum := int(anonClassType.StartPosition().Row) + 1
				syntheticName := fmt.Sprintf("%s$anon%d", typeName, lineNum)

				endLine := 0
				if anonClassDef, ok := match.Captures["anon_class.def"]; ok {
					endLine = int(anonClassDef.EndPosition().Row) + 1
				}

				// Create a symbol for the anonymous class itself
				symbol := &model.Symbol{
					Name:       syntheticName,
					Kind:       model.SymbolKindClass,
					File:       filePath,
					Line:       lineNum,
					EndLine:    endLine,
					Column:     int(anonClassType.StartPosition().Column) + 1,
					Language:   model.LanguageJava,
					Package:    fileSymbols.Package,
					IsExported: false, // Anonymous classes are never exported
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}
		}

		// #13: assign Owner (enclosing class/interface/enum/record) to methods
		// so same-named methods in one package get distinct qualified names
		a.assignJavaOwners(fileSymbols)

		return nil // Success
	})
	if err != nil {
		return nil, err
	}
	return fileSymbols, nil
}

func (a *JavaAdapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	var calls []*model.CallSite
	err := treesitter.WithParser(model.LanguageJava, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, javaCallsQuery)
		if err != nil {
			return err
		}

		calls = make([]*model.CallSite, 0)

		// Best-effort symbol parse for implicit-this hints: an unqualified call
		// (bare `greet(...)` or `this.greet(...)`) targets the enclosing class,
		// so its callee carries that class as owner hint. A failed parse only
		// means no hints; call extraction still succeeds.
		var parsed *model.FileSymbols
		if fs, perr := a.ParseFile(filePath); perr == nil {
			parsed = fs
		}

		for _, match := range matches {
			callSite := &model.CallSite{
				File:     filePath,
				Resolved: true,
			}

			if methodNode, ok := match.Captures["call.method"]; ok {
				callSite.Line = int(methodNode.StartPosition().Row) + 1
				callSite.Column = int(methodNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(methodNode, content),
					Kind:     model.SymbolKindMethod,
					Language: model.LanguageJava,
				}
				// Receiver owner hint: userService.find() -> Owner "userService".
				// Best-effort (variable/field name, not a type); see receiverHint.
				if objNode, ok := match.Captures["call.object"]; ok {
					callSite.Callee.Owner = receiverHint(objNode, content)
					if callSite.Callee.Owner == "" && objNode.Kind() == "this" && parsed != nil {
						// Explicit this receiver: the target is the enclosing class.
						callSite.Callee.Owner = enclosingClassName(parsed.Symbols, callSite.Line)
					}
				} else if parsed != nil {
					// Unqualified call: implicit this, so hint the enclosing class.
					callSite.Callee.Owner = enclosingClassName(parsed.Symbols, callSite.Line)
				}
				calls = append(calls, callSite)
			}

			// Constructor calls - new ClassName(...) (#7)
			if ctorTypeNode, ok := match.Captures["call.constructor_type"]; ok {
				callSite.Line = int(ctorTypeNode.StartPosition().Row) + 1
				callSite.Column = int(ctorTypeNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(ctorTypeNode, content),
					Kind:     model.SymbolKindClass,
					Language: model.LanguageJava,
				}
				calls = append(calls, callSite)
			}

			// Explicit constructor invocations - super(...) or this(...) (#7)
			if superThis, ok := match.Captures["call.super_this"]; ok {
				callSite.Line = int(superThis.StartPosition().Row) + 1
				callSite.Column = int(superThis.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     "<init>",
					Kind:     model.SymbolKindMethod,
					Language: model.LanguageJava,
				}
				calls = append(calls, callSite)
			}
		}

		// #22: Query symbols for inheritance edges
		symbolMatches, err := parser.Query(tree, javaSymbolsQuery)
		if err != nil {
			return err
		}

		for _, match := range symbolMatches {
			callSite := &model.CallSite{
				File:     filePath,
				Resolved: true,
			}

			// Inheritance edges - superclass
			if superclass, ok := match.Captures["class.superclass"]; ok {
				callSite.Line = int(superclass.StartPosition().Row) + 1
				callSite.Column = int(superclass.StartPosition().Column) + 1
				callSite.EdgeKind = model.EdgeKindInheritance
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(superclass, content),
					Kind:     model.SymbolKindClass,
					Language: model.LanguageJava,
				}
				calls = append(calls, callSite)
			}

			// Inheritance edges - interfaces
			if iface, ok := match.Captures["class.interface"]; ok {
				callSite.Line = int(iface.StartPosition().Row) + 1
				callSite.Column = int(iface.StartPosition().Column) + 1
				callSite.EdgeKind = model.EdgeKindInheritance
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(iface, content),
					Kind:     model.SymbolKindInterface,
					Language: model.LanguageJava,
				}
				calls = append(calls, callSite)
			}

			// Interface inheritance edges
			if ifaceExtends, ok := match.Captures["interface.extends"]; ok {
				callSite.Line = int(ifaceExtends.StartPosition().Row) + 1
				callSite.Column = int(ifaceExtends.StartPosition().Column) + 1
				callSite.EdgeKind = model.EdgeKindInheritance
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(ifaceExtends, content),
					Kind:     model.SymbolKindInterface,
					Language: model.LanguageJava,
				}
				calls = append(calls, callSite)
			}

			// Field type-ref edges (#9)
			if fieldType, ok := match.Captures["field.type"]; ok {
				typeText := treesitter.GetNodeText(fieldType, content)
				typeName := a.stripJavaTypeArguments(typeText)
				if a.isUserJavaType(typeName) {
					callSite.Line = int(fieldType.StartPosition().Row) + 1
					callSite.Column = int(fieldType.StartPosition().Column) + 1
					callSite.EdgeKind = model.EdgeKindTypeRef
					callSite.Callee = &model.Symbol{
						Name:     typeName,
						Kind:     model.SymbolKindClass,
						Language: model.LanguageJava,
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

func (a *JavaAdapter) IsTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	return strings.HasSuffix(base, "Test.java") || strings.HasSuffix(base, "Tests.java")
}

func (a *JavaAdapter) IsExported(symbolName string, context interface{}) bool {
	// In Java, check for public modifier
	if modifiers, ok := context.(string); ok {
		return strings.Contains(modifiers, "public")
	}
	return false
}

func (a *JavaAdapter) ResolveImports(filePath string) ([]string, error) {
	fileSymbols, err := a.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file for imports: %w", err)
	}
	return fileSymbols.Imports, nil
}

// extractTypeParameters extracts type parameter names from a TreeSitter match
// For Java: type_parameters contains type_parameter nodes like <T, U extends Comparable<U>>
func (a *JavaAdapter) extractTypeParameters(match *treesitter.QueryMatch, captureKey string, content []byte) []string {
	node, ok := match.Captures[captureKey]
	if !ok {
		return nil
	}

	text := treesitter.GetNodeText(node, content)
	if text == "" {
		return nil
	}

	// Parse type parameters from text like "<T>" or "<T, U>" or "<T extends Foo, U>"
	// Remove angle brackets
	text = strings.TrimPrefix(text, "<")
	text = strings.TrimSuffix(text, ">")

	var typeParams []string
	parts := strings.Split(text, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Extract just the type parameter name (before extends/super)
		if idx := strings.Index(part, " extends "); idx != -1 {
			part = part[:idx]
		}
		if idx := strings.Index(part, " super "); idx != -1 {
			part = part[:idx]
		}
		part = strings.TrimSpace(part)
		if part != "" {
			typeParams = append(typeParams, part)
		}
	}

	return typeParams
}

// stripJavaTypeArguments removes generic type arguments from type names (#9)
// Example: "List<String>" -> "List", "Map<K,V>" -> "Map"
func (a *JavaAdapter) stripJavaTypeArguments(typeText string) string {
	// Find first < and remove everything after
	if idx := strings.Index(typeText, "<"); idx != -1 {
		typeText = typeText[:idx]
	}
	// Trim array brackets
	typeText = strings.TrimSuffix(typeText, "[]")
	return strings.TrimSpace(typeText)
}

// isUserJavaType checks if a type name is a user-defined type (#9)
// Returns false for primitives and common java.lang/java.util types
func (a *JavaAdapter) isUserJavaType(typeName string) bool {
	if typeName == "" {
		return false
	}
	// Primitives
	switch typeName {
	case "boolean", "byte", "char", "short", "int", "long", "float", "double", "void":
		return false
	}
	// Common java.lang types
	switch typeName {
	case "String", "Object", "Class", "Integer", "Long", "Double", "Float",
		"Boolean", "Character", "Byte", "Short", "Number", "Void",
		"StringBuilder", "StringBuffer", "Thread", "Runnable", "Exception",
		"RuntimeException", "Error", "Throwable":
		return false
	}
	// Common java.util types
	switch typeName {
	case "List", "ArrayList", "LinkedList", "Set", "HashSet", "TreeSet",
		"Map", "HashMap", "TreeMap", "LinkedHashMap", "Queue", "Deque",
		"Collection", "Iterator", "Optional", "Stream":
		return false
	}
	return true
}
