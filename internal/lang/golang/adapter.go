package golang

import (
	_ "embed"
	"fmt"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/lang/golang/godeep"
	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/ssa"
)

//go:embed go_symbols.scm
var goSymbolsQuery string

//go:embed go_calls.scm
var goCallsQuery string

func init() {
	lang.Register(&GoAdapter{})
}

// GoAdapter implements the LanguageAdapter interface for Go
type GoAdapter struct{}

func (a *GoAdapter) Language() model.Language {
	return model.LanguageGo
}

func (a *GoAdapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	var fileSymbols *model.FileSymbols
	err := treesitter.WithParser(model.LanguageGo, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, goSymbolsQuery)
		if err != nil {
			return err
		}

		fileSymbols = &model.FileSymbols{
			File:     filePath,
			Language: model.LanguageGo,
			Symbols:  make([]*model.Symbol, 0),
			Imports:  make([]string, 0),
		}

		for _, match := range matches {
			if pkgNode, ok := match.Captures["package.name"]; ok {
				fileSymbols.Package = treesitter.GetNodeText(pkgNode, content)
			}

			if importPath, ok := match.Captures["import.path"]; ok {
				path := treesitter.GetNodeText(importPath, content)
				path = strings.Trim(path, "\"")
				fileSymbols.Imports = append(fileSymbols.Imports, path)
			}

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
					Language:       model.LanguageGo,
					Package:        fileSymbols.Package,
					TypeParameters: a.extractTypeParameters(match, "function.type_params", content),
					IsExported:     a.IsExported(name, nil),
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Method declarations
			if methodName, ok := match.Captures["method.name"]; ok {
				name := treesitter.GetNodeText(methodName, content)
				endLine := 0
				if methodDef, ok := match.Captures["method.def"]; ok {
					endLine = int(methodDef.EndPosition().Row) + 1
				}
				// #12: Extract receiver type as Owner
				owner := ""
				if methodReceiver, ok := match.Captures["method.receiver"]; ok {
					recvText := treesitter.GetNodeText(methodReceiver, content)
					owner = a.stripGoReceiverType(recvText)
				}
				symbol := &model.Symbol{
					Name:       name,
					Owner:      owner,
					Kind:       model.SymbolKindMethod,
					File:       filePath,
					Line:       int(methodName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(methodName.StartPosition().Column) + 1,
					Language:   model.LanguageGo,
					Package:    fileSymbols.Package,
					IsExported: a.IsExported(name, nil),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Type declarations
			if typeName, ok := match.Captures["type.name"]; ok {
				kind := model.SymbolKindStruct
				if _, isInterface := match.Captures["type.interface"]; isInterface {
					kind = model.SymbolKindInterface
				}
				endLine := 0
				if typeDef, ok := match.Captures["type.def"]; ok {
					endLine = int(typeDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:           treesitter.GetNodeText(typeName, content),
					Kind:           kind,
					File:           filePath,
					Line:           int(typeName.StartPosition().Row) + 1,
					EndLine:        endLine,
					Column:         int(typeName.StartPosition().Column) + 1,
					Language:       model.LanguageGo,
					Package:        fileSymbols.Package,
					TypeParameters: a.extractTypeParameters(match, "type.type_params", content),
					IsExported:     a.IsExported(treesitter.GetNodeText(typeName, content), nil),
					IsTest:         a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Const declarations
			if constName, ok := match.Captures["const.name"]; ok {
				name := treesitter.GetNodeText(constName, content)
				endLine := int(constName.EndPosition().Row) + 1
				if constDef, ok := match.Captures["const.def"]; ok {
					endLine = int(constDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindConstant,
					File:       filePath,
					Line:       int(constName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(constName.StartPosition().Column) + 1,
					Language:   model.LanguageGo,
					Package:    fileSymbols.Package,
					IsExported: a.IsExported(name, nil),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Var declarations
			if varName, ok := match.Captures["var.name"]; ok {
				name := treesitter.GetNodeText(varName, content)
				endLine := int(varName.EndPosition().Row) + 1
				if varDef, ok := match.Captures["var.def"]; ok {
					endLine = int(varDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindVariable,
					File:       filePath,
					Line:       int(varName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(varName.StartPosition().Column) + 1,
					Language:   model.LanguageGo,
					Package:    fileSymbols.Package,
					IsExported: a.IsExported(name, nil),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Closure/anonymous function declarations (#20)
			if closureDef, ok := match.Captures["closure.def"]; ok {
				line := int(closureDef.StartPosition().Row) + 1
				endLine := int(closureDef.EndPosition().Row) + 1
				// Generate a synthetic name for the closure: func$line:column
				syntheticName := fmt.Sprintf("func$%d:%d", line, int(closureDef.StartPosition().Column)+1)
				symbol := &model.Symbol{
					Name:       syntheticName,
					Kind:       model.SymbolKindFunction,
					File:       filePath,
					Line:       line,
					EndLine:    endLine,
					Column:     int(closureDef.StartPosition().Column) + 1,
					Language:   model.LanguageGo,
					Package:    fileSymbols.Package,
					IsExported: false, // Closures are never exported
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

func (a *GoAdapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	var calls []*model.CallSite
	err := treesitter.WithParser(model.LanguageGo, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, goCallsQuery)
		if err != nil {
			return err
		}

		calls = make([]*model.CallSite, 0)

		// Selector fields that are the function of a call expression; the
		// selector captures below skip these to avoid duplicate edges.
		calledFields := make(map[uint]bool)
		for _, match := range matches {
			if methodNode, ok := match.Captures["call.method"]; ok {
				calledFields[methodNode.StartByte()] = true
			}
		}

		for _, match := range matches {
			callSite := &model.CallSite{
				File:     filePath,
				Resolved: true,
			}

			if funcNode, ok := match.Captures["call.function"]; ok {
				callSite.Line = int(funcNode.StartPosition().Row) + 1
				callSite.Column = int(funcNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(funcNode, content),
					Language: model.LanguageGo,
				}
			}

			if methodNode, ok := match.Captures["call.method"]; ok {
				callSite.Line = int(methodNode.StartPosition().Row) + 1
				callSite.Column = int(methodNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(methodNode, content),
					Kind:     model.SymbolKindMethod,
					Language: model.LanguageGo,
				}
				// Receiver owner hint: userService.Find() -> Owner "userService".
				// Best-effort (variable name, not a resolved type).
				if operandNode, ok := match.Captures["call.operand"]; ok && operandNode.Kind() == "identifier" {
					callSite.Callee.Owner = treesitter.GetNodeText(operandNode, content)
				}
			}

			// Selector in a non-call position (method value, e.g. f := s.Process)
			if fieldNode, ok := match.Captures["selector.field"]; ok && !calledFields[fieldNode.StartByte()] {
				callSite.Line = int(fieldNode.StartPosition().Row) + 1
				callSite.Column = int(fieldNode.StartPosition().Column) + 1
				callSite.Callee = &model.Symbol{
					Name:     treesitter.GetNodeText(fieldNode, content),
					Kind:     model.SymbolKindMethod,
					Language: model.LanguageGo,
				}
				if operandNode, ok := match.Captures["selector.operand"]; ok && operandNode.Kind() == "identifier" {
					callSite.Callee.Owner = treesitter.GetNodeText(operandNode, content)
				}
			}

			if typeNode, ok := match.Captures["type.ref"]; ok {
				name := treesitter.GetNodeText(typeNode, content)
				if a.isUserTypeName(name) {
					callSite.Line = int(typeNode.StartPosition().Row) + 1
					callSite.Column = int(typeNode.StartPosition().Column) + 1
					callSite.EdgeKind = model.EdgeKindTypeRef
					callSite.Callee = &model.Symbol{
						Name:     name,
						Kind:     model.SymbolKindStruct,
						Language: model.LanguageGo,
					}
				}
			}

			if typeNode, ok := match.Captures["type.assertion"]; ok {
				name := treesitter.GetNodeText(typeNode, content)
				if a.isUserTypeName(name) {
					callSite.Line = int(typeNode.StartPosition().Row) + 1
					callSite.Column = int(typeNode.StartPosition().Column) + 1
					callSite.EdgeKind = model.EdgeKindTypeRef
					callSite.Callee = &model.Symbol{
						Name:     name,
						Kind:     model.SymbolKindInterface,
						Language: model.LanguageGo,
					}
				}
			}

			// Qualified type references (e.g., time.Time, context.Context) - #5
			if pkgNode, ok := match.Captures["typeref.pkg"]; ok {
				if nameNode, ok := match.Captures["typeref.name"]; ok {
					pkg := treesitter.GetNodeText(pkgNode, content)
					name := treesitter.GetNodeText(nameNode, content)
					// Use the name node for position (the actual type name)
					callSite.Line = int(nameNode.StartPosition().Row) + 1
					callSite.Column = int(nameNode.StartPosition().Column) + 1
					callSite.EdgeKind = model.EdgeKindTypeRef
					callSite.Callee = &model.Symbol{
						Package:  pkg, // Local import alias
						Name:     name,
						Kind:     model.SymbolKindStruct,
						Language: model.LanguageGo,
					}
				}
			}

			if callSite.Callee != nil {
				if callSite.EdgeKind == "" {
					callSite.EdgeKind = model.EdgeKindCall
				}
				calls = append(calls, callSite)
			}
		}

		return nil // Success
	})
	if err != nil {
		return nil, err
	}

	// Multiple query patterns can capture the same type reference (e.g. a
	// qualified pointer field matches both the field and the generic
	// pointer patterns); emit each distinct edge once.
	type callKey struct {
		edgeKind         model.EdgeKind
		name, pkg, owner string
		line, col        int
	}
	seen := make(map[callKey]bool, len(calls))
	deduped := make([]*model.CallSite, 0, len(calls))
	for _, call := range calls {
		key := callKey{call.EdgeKind, call.Callee.Name, call.Callee.Package, call.Callee.Owner, call.Line, call.Column}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, call)
	}

	return deduped, nil
}

func (a *GoAdapter) IsTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	return strings.HasSuffix(base, "_test.go")
}

func (a *GoAdapter) IsExported(symbolName string, context interface{}) bool {
	if len(symbolName) == 0 {
		return false
	}
	// In Go, exported symbols start with uppercase letter
	firstRune := []rune(symbolName)[0]
	return unicode.IsUpper(firstRune)
}

func (a *GoAdapter) ResolveImports(filePath string) ([]string, error) {
	// Use standard Go parser for more reliable import extraction
	// TreeSitter query has known limitations capturing all import formats
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file for imports: %w", err)
	}

	var imports []string
	for _, imp := range file.Imports {
		if imp.Path != nil {
			// imp.Path.Value includes quotes, strip them
			path := strings.Trim(imp.Path.Value, `"`)
			imports = append(imports, path)
		}
	}

	return imports, nil
}

// isTestSymbolName checks if a symbol name looks like a test function
func (a *GoAdapter) isTestSymbolName(name string) bool {
	return strings.HasPrefix(name, "Test") ||
		strings.HasPrefix(name, "Benchmark") ||
		strings.HasPrefix(name, "Example")
}

func (a *GoAdapter) isUserTypeName(name string) bool {
	if name == "" {
		return false
	}
	switch name {
	case "any", "bool", "byte", "comparable", "complex64", "complex128", "error",
		"float32", "float64", "int", "int8", "int16", "int32", "int64",
		"rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return false
	default:
		return true
	}
}

// extractTypeParameters extracts type parameter names from a TreeSitter match
// For Go: type_parameter_list contains type_parameter_declaration nodes
// #28: Improved to handle complex type constraints like "T interface{Method()}"
func (a *GoAdapter) extractTypeParameters(match *treesitter.QueryMatch, captureKey string, content []byte) []string {
	node, ok := match.Captures[captureKey]
	if !ok {
		return nil
	}

	text := treesitter.GetNodeText(node, content)
	if text == "" {
		return nil
	}

	// Parse type parameters from text like "[T any]" or "[T, U any]" or "[T comparable, U any]"
	// Also handle complex constraints: "[T interface{Method()}, U ~int|~string]"
	// Remove brackets
	text = strings.TrimPrefix(text, "[")
	text = strings.TrimSuffix(text, "]")

	var typeParams []string

	// Handle complex constraints with braces and pipes
	// Split carefully to avoid breaking on commas inside interface{} or union types
	inBraces := 0
	current := strings.Builder{}

	for _, ch := range text {
		switch ch {
		case '{':
			inBraces++
			current.WriteRune(ch)
		case '}':
			inBraces--
			current.WriteRune(ch)
		case ',':
			if inBraces == 0 {
				// This is a separator between type parameters
				param := a.extractSingleTypeParam(current.String())
				if param != "" {
					typeParams = append(typeParams, param)
				}
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Don't forget the last parameter
	if current.Len() > 0 {
		param := a.extractSingleTypeParam(current.String())
		if param != "" {
			typeParams = append(typeParams, param)
		}
	}

	return typeParams
}

// extractSingleTypeParam extracts the type parameter name from a parameter declaration (#28)
// Examples: "T any" -> "T", "U comparable" -> "U", "V interface{Method()}" -> "V", "W ~int|~string" -> "W"
func (a *GoAdapter) extractSingleTypeParam(paramDecl string) string {
	paramDecl = strings.TrimSpace(paramDecl)
	if paramDecl == "" {
		return ""
	}

	// The name is always the first token before any constraint
	fields := strings.Fields(paramDecl)
	if len(fields) > 0 {
		return fields[0]
	}

	return ""
}

// stripGoReceiverType extracts the receiver type from a method receiver (#12)
// Example: "(s *Server)" -> "Server", "(s Server[T])" -> "Server"
func (a *GoAdapter) stripGoReceiverType(recvText string) string {
	// Remove parentheses
	recvText = strings.TrimPrefix(recvText, "(")
	recvText = strings.TrimSuffix(recvText, ")")
	recvText = strings.TrimSpace(recvText)

	// Split by whitespace to separate name from type
	parts := strings.Fields(recvText)
	if len(parts) < 1 {
		return ""
	}

	// Get the type (last part; an unnamed receiver like "(T)" is the type
	// itself), remove pointer if present
	typeName := parts[len(parts)-1]
	typeName = strings.TrimPrefix(typeName, "*")

	// Remove generic type parameters if present
	if idx := strings.Index(typeName, "["); idx != -1 {
		typeName = typeName[:idx]
	}

	return typeName
}

// BuildPackageCallGraph builds precise call edges using SSA/RTA (#21)
func (a *GoAdapter) BuildPackageCallGraph(pkgDir string, buildTags, customEntryPoints []string) ([]*model.CallSite, error) {
	analyzer := godeep.NewAnalyzer()

	// Load packages for the directory
	if err := analyzer.LoadPackages(pkgDir); err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}

	// Build SSA
	if err := analyzer.BuildSSA(); err != nil {
		return nil, fmt.Errorf("failed to build SSA: %w", err)
	}

	// Build call graph with optional custom entry points
	cg, err := analyzer.BuildCallGraph(customEntryPoints...)
	if err != nil {
		return nil, fmt.Errorf("failed to build call graph: %w", err)
	}

	// Convert callgraph edges to CallSite format
	var callSites []*model.CallSite
	for _, node := range cg.Nodes {
		if node.Func == nil {
			continue
		}

		caller := a.ssaFunctionToSymbol(analyzer, node.Func)
		if caller == nil {
			continue
		}

		for _, edge := range node.Out {
			if edge.Callee == nil || edge.Callee.Func == nil {
				continue
			}

			callee := a.ssaFunctionToSymbol(analyzer, edge.Callee.Func)
			if callee == nil {
				continue
			}

			// Get position info from the call site
			pos := analyzer.Prog.Fset.Position(edge.Pos())

			callSite := &model.CallSite{
				File:     pos.Filename,
				Line:     pos.Line,
				Column:   pos.Column,
				EdgeKind: model.EdgeKindCall,
				Resolved: true,
				Callee:   callee,
			}

			callSites = append(callSites, callSite)
		}
	}

	return callSites, nil
}

// ssaFunctionToSymbol converts an SSA function to our Symbol model (#21)
// Must set Owner field for consistency with AST-based symbols
func (a *GoAdapter) ssaFunctionToSymbol(analyzer *godeep.Analyzer, fn *ssa.Function) *model.Symbol {
	fn = godeep.SourceFunction(fn)
	if fn == nil {
		return nil
	}

	pkg := fn.Pkg
	if pkg == nil {
		return nil
	}

	pos := analyzer.Prog.Fset.Position(fn.Pos())

	kind := model.SymbolKindFunction
	var owner string
	if recv := fn.Signature.Recv(); recv != nil {
		kind = model.SymbolKindMethod
		// Extract receiver type for Owner field (#15 consistency)
		recvType := recv.Type()
		if ptr, ok := recvType.(*types.Pointer); ok {
			recvType = ptr.Elem()
		}
		if named, ok := recvType.(*types.Named); ok {
			owner = named.Obj().Name()
		}
	}

	isExported := fn.Object() != nil && fn.Object().Exported()

	return &model.Symbol{
		Name:       fn.Name(),
		Kind:       kind,
		File:       pos.Filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Language:   model.LanguageGo,
		Package:    pkg.Pkg.Path(),
		Owner:      owner, // #15: Set Owner for method symbols
		IsExported: isExported,
	}
}
