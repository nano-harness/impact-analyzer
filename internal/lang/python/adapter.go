package python

import (
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/lang/treesitter"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func init() { lang.Register(&Adapter{}) }

//go:embed py_symbols.scm
var pySymbolsQuery string

//go:embed py_calls.scm
var pyCallsQuery string

// Regex patterns for parsing __all__ export list
var (
	allRe    = regexp.MustCompile(`__all__\s*=\s*\[([^\]]*)\]`)
	stringRe = regexp.MustCompile(`["']([^"']+)["']`)
)

type Adapter struct{}

func (a *Adapter) Language() model.Language { return model.LanguagePython }

func (a *Adapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	var fileSymbols *model.FileSymbols
	err := treesitter.WithParser(model.LanguagePython, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, pySymbolsQuery)
		if err != nil {
			return err
		}

		pkg := pythonModuleName(filePath)
		fileSymbols = &model.FileSymbols{
			File:     filePath,
			Language: model.LanguagePython,
			Package:  pkg,
			Symbols:  make([]*model.Symbol, 0),
			Imports:  make([]string, 0),
		}

		// Parse __all__ for export information
		exportedByAll := parseAll(content)

		for _, match := range matches {
			// Function definitions
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
					Language:   model.LanguagePython,
					Package:    pkg,
					IsExported: a.isExportedWithAll(name, exportedByAll),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Class definitions
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
					Language:   model.LanguagePython,
					Package:    pkg,
					IsExported: a.isExportedWithAll(name, exportedByAll),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}

			// Import statements
			if importModule, ok := match.Captures["import.module"]; ok {
				module := treesitter.GetNodeText(importModule, content)
				fileSymbols.Imports = append(fileSymbols.Imports, module)
			}

			// Import from statements
			if fromModule, ok := match.Captures["import.from_module"]; ok {
				module := treesitter.GetNodeText(fromModule, content)
				fileSymbols.Imports = append(fileSymbols.Imports, module)
			}

			// Import from relative statements (.X or ..X)
			if fromRelative, ok := match.Captures["import.from_relative"]; ok {
				module := treesitter.GetNodeText(fromRelative, content)
				fileSymbols.Imports = append(fileSymbols.Imports, module)
			}

			// Module-level assignments binding a call result (app = FastAPI())
			if varName, ok := match.Captures["variable.name"]; ok {
				name := treesitter.GetNodeText(varName, content)
				endLine := 0
				if varDef, ok := match.Captures["variable.def"]; ok {
					endLine = int(varDef.EndPosition().Row) + 1
				}
				symbol := &model.Symbol{
					Name:       name,
					Kind:       model.SymbolKindVariable,
					File:       filePath,
					Line:       int(varName.StartPosition().Row) + 1,
					EndLine:    endLine,
					Column:     int(varName.StartPosition().Column) + 1,
					Language:   model.LanguagePython,
					Package:    pkg,
					IsExported: a.isExportedWithAll(name, exportedByAll),
					IsTest:     a.IsTestFile(filePath),
				}
				fileSymbols.Symbols = append(fileSymbols.Symbols, symbol)
			}
		}

		// #14: Assign owners to methods
		// Read lines for owner assignment (still uses line-based logic)
		lines := strings.Split(string(content), "\n")
		assignPythonOwners(fileSymbols, lines)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return fileSymbols, nil
}

func (a *Adapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	var calls []*model.CallSite
	err := treesitter.WithParser(model.LanguagePython, func(parser *treesitter.Parser) error {
		tree, err := parser.ParseFile(filePath)
		if err != nil {
			return err
		}
		defer tree.Close()

		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		matches, err := parser.Query(tree, pyCallsQuery)
		if err != nil {
			return err
		}

		calls = make([]*model.CallSite, 0)
		for _, match := range matches {
			callSite := &model.CallSite{
				File:     filePath,
				Resolved: true,
			}

			// Function calls
			if funcCall, ok := match.Captures["call.function"]; ok {
				name := treesitter.GetNodeText(funcCall, content)
				// Skip Python keywords
				if !isKeywordLike(name) && !isDynamicBuiltin(name) {
					callSite.Line = int(funcCall.StartPosition().Row) + 1
					callSite.Column = int(funcCall.StartPosition().Column) + 1
					callSite.Callee = &model.Symbol{
						Name:     name,
						Language: model.LanguagePython,
					}
					calls = append(calls, callSite)
				}
			}

			// Method calls
			if methodCall, ok := match.Captures["call.method"]; ok {
				name := treesitter.GetNodeText(methodCall, content)
				if !isKeywordLike(name) && !isDynamicBuiltin(name) {
					callSite.Line = int(methodCall.StartPosition().Row) + 1
					callSite.Column = int(methodCall.StartPosition().Column) + 1
					callSite.Callee = &model.Symbol{
						Name:     name,
						Kind:     model.SymbolKindMethod,
						Language: model.LanguagePython,
					}
					// Receiver owner hint: userService.find() -> Owner "userService".
					// Best-effort (variable/attribute name, not a type); see receiverHint.
					if objNode, ok := match.Captures["call.object"]; ok {
						callSite.Callee.Owner = receiverHint(objNode, content)
					}
					calls = append(calls, callSite)
				}
			}

			// Await calls
			if awaitCall, ok := match.Captures["call.await_function"]; ok {
				name := treesitter.GetNodeText(awaitCall, content)
				if !isKeywordLike(name) && !isDynamicBuiltin(name) {
					callSite.Line = int(awaitCall.StartPosition().Row) + 1
					callSite.Column = int(awaitCall.StartPosition().Column) + 1
					callSite.Callee = &model.Symbol{
						Name:     name,
						Language: model.LanguagePython,
					}
					calls = append(calls, callSite)
				}
			}
		}

		// #25: Extract decorator usage
		symbolMatches, err := parser.Query(tree, pySymbolsQuery)
		if err != nil {
			return err
		}

		for _, match := range symbolMatches {
			// Inheritance edges - base classes (#22, mirrors Java)
			if supers, ok := match.Captures["class.superclasses"]; ok {
				for i := uint(0); i < supers.NamedChildCount(); i++ {
					arg := supers.NamedChild(i)
					if arg.Kind() != "identifier" && arg.Kind() != "attribute" {
						continue
					}
					name := treesitter.GetNodeText(arg, content)
					if name == "" || name == "object" {
						continue
					}
					calls = append(calls, &model.CallSite{
						File:     filePath,
						Line:     int(arg.StartPosition().Row) + 1,
						Column:   int(arg.StartPosition().Column) + 1,
						Resolved: true,
						EdgeKind: model.EdgeKindInheritance,
						Callee: &model.Symbol{
							Name:     name,
							Kind:     model.SymbolKindClass,
							Language: model.LanguagePython,
						},
					})
				}
			}

			// Decorator usage
			if decorator, ok := match.Captures["decorator.def"]; ok {
				decoratorText := treesitter.GetNodeText(decorator, content)
				// Remove @ prefix
				decoratorName := strings.TrimPrefix(decoratorText, "@")
				// Extract the decorator name (without arguments)
				// e.g., @app.route("/api") -> app.route
				if idx := strings.Index(decoratorName, "("); idx != -1 {
					decoratorName = decoratorName[:idx]
				}
				decoratorName = strings.TrimSpace(decoratorName)

				if !isKeywordLike(decoratorName) && decoratorName != "" {
					callSite := &model.CallSite{
						File:     filePath,
						Line:     int(decorator.StartPosition().Row) + 1,
						Column:   int(decorator.StartPosition().Column) + 1,
						Resolved: true,
						EdgeKind: model.EdgeKindDecorator,
						Callee: &model.Symbol{
							Name:     decoratorName,
							Language: model.LanguagePython,
						},
					}
					calls = append(calls, callSite)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return calls, nil
}

// Helper function to check if a name is a Python keyword
func isKeywordLike(name string) bool {
	keywords := map[string]bool{
		"if": true, "for": true, "while": true, "with": true,
		"return": true, "raise": true, "except": true,
		"class": true, "def": true, "async": true,
	}
	return keywords[name]
}

func (a *Adapter) IsTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	dir := filepath.ToSlash(filepath.Dir(filePath))
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") || base == "conftest.py" || strings.Contains(dir, "/tests") || strings.HasPrefix(dir, "tests")
}

func (a *Adapter) IsExported(symbolName string, context interface{}) bool {
	return !strings.HasPrefix(symbolName, "_")
}
func (a *Adapter) ResolveImports(filePath string) ([]string, error) {
	fs, err := a.ParseFile(filePath)
	if err != nil {
		return nil, err
	}
	return fs.Imports, nil
}

func (a *Adapter) isExportedWithAll(name string, all map[string]bool) bool {
	if all != nil {
		return all[name]
	}
	return a.IsExported(name, nil)
}

func parseAll(content []byte) map[string]bool {
	m := allRe.FindSubmatch(content)
	if m == nil {
		return nil
	}
	out := map[string]bool{}
	for _, sm := range stringRe.FindAllStringSubmatch(string(m[1]), -1) {
		out[sm[1]] = true
	}
	return out
}

func endLine(lines []string, start int) int {
	startIndent := indent(lines[start])
	last := start + 1
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent(lines[i]) <= startIndent {
			break
		}
		last = i + 1
	}
	return last
}

func indent(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }

func pythonModuleName(filePath string) string {
	// #26: Better module name resolution
	// Convert file path to Python module path
	// e.g., /path/to/mypackage/submodule/file.py -> mypackage.submodule.file
	// e.g., /path/to/mypackage/submodule/__init__.py -> mypackage.submodule

	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	dir := filepath.Dir(filePath)

	// Build module path by walking up directories that contain __init__.py
	parts := []string{}
	if base != "__init__" {
		parts = append(parts, base)
	}

	// Walk up the directory tree while we find __init__.py files
	for {
		parentDir := filepath.Dir(dir)
		if parentDir == dir || parentDir == "" {
			break
		}

		dirName := filepath.Base(dir)
		initFile := filepath.Join(dir, "__init__.py")

		// Check if __init__.py exists
		if _, err := os.Stat(initFile); err == nil {
			parts = append([]string{dirName}, parts...)
			dir = parentDir
		} else {
			// No __init__.py, so we've reached the top of the package
			break
		}
	}

	if len(parts) == 0 {
		return base
	}
	return strings.Join(parts, ".")
}

func isDynamicBuiltin(name string) bool {
	switch name {
	case "getattr", "setattr", "delattr", "hasattr", "__import__":
		return true
	default:
		return false
	}
}

// stripPythonCode strips comments and string literals from Python code (#10)
// Returns content with same length, replacing non-code regions with spaces
func stripPythonCode(content string) string {
	var result strings.Builder
	result.Grow(len(content))

	type state int
	const (
		stateCode state = iota
		stateLineComment
		stateSingleQuote
		stateDoubleQuote
		stateTripleSingle
		stateTripleDouble
	)

	currentState := stateCode
	i := 0
	for i < len(content) {
		ch := content[i]

		switch currentState {
		case stateCode:
			// Check for comments
			if ch == '#' {
				result.WriteByte(' ')
				currentState = stateLineComment
				i++
				continue
			}
			// Check for triple-quoted strings
			if i+2 < len(content) && content[i:i+3] == "\"\"\"" {
				result.WriteString("   ")
				currentState = stateTripleDouble
				i += 3
				continue
			}
			if i+2 < len(content) && content[i:i+3] == "'''" {
				result.WriteString("   ")
				currentState = stateTripleSingle
				i += 3
				continue
			}
			// Check for single/double quoted strings
			if ch == '"' {
				result.WriteByte(' ')
				currentState = stateDoubleQuote
				i++
				continue
			}
			if ch == '\'' {
				result.WriteByte(' ')
				currentState = stateSingleQuote
				i++
				continue
			}
			// Regular code
			result.WriteByte(ch)
			i++

		case stateLineComment:
			if ch == '\n' {
				result.WriteByte('\n')
				currentState = stateCode
			} else {
				result.WriteByte(' ')
			}
			i++

		case stateSingleQuote:
			if ch == '\\' && i+1 < len(content) {
				// Escape sequence - skip both chars
				result.WriteString("  ")
				i += 2
			} else if ch == '\'' {
				result.WriteByte(' ')
				currentState = stateCode
				i++
			} else if ch == '\n' {
				// Unclosed string - back to code (syntax error, but be permissive)
				result.WriteByte('\n')
				currentState = stateCode
				i++
			} else {
				result.WriteByte(' ')
				i++
			}

		case stateDoubleQuote:
			if ch == '\\' && i+1 < len(content) {
				// Escape sequence - skip both chars
				result.WriteString("  ")
				i += 2
			} else if ch == '"' {
				result.WriteByte(' ')
				currentState = stateCode
				i++
			} else if ch == '\n' {
				// Unclosed string - back to code (syntax error, but be permissive)
				result.WriteByte('\n')
				currentState = stateCode
				i++
			} else {
				result.WriteByte(' ')
				i++
			}

		case stateTripleSingle:
			if i+2 < len(content) && content[i:i+3] == "'''" {
				result.WriteString("   ")
				currentState = stateCode
				i += 3
			} else if ch == '\n' {
				result.WriteByte('\n')
				i++
			} else {
				result.WriteByte(' ')
				i++
			}

		case stateTripleDouble:
			if i+2 < len(content) && content[i:i+3] == "\"\"\"" {
				result.WriteString("   ")
				currentState = stateCode
				i += 3
			} else if ch == '\n' {
				result.WriteByte('\n')
				i++
			} else {
				result.WriteByte(' ')
				i++
			}
		}
	}

	return result.String()
}
