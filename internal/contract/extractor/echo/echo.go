package echo

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// EchoExtractor extracts HTTP route contracts from labstack/echo applications
type EchoExtractor struct{}

// Name returns the extractor name
func (e *EchoExtractor) Name() string {
	return "echo"
}

// Kind returns the contract kind
func (e *EchoExtractor) Kind() model.ContractKind {
	return model.ContractKindHTTPRoute
}

// Language returns the supported language
func (e *EchoExtractor) Language() model.Language {
	return model.LanguageGo
}

// CanHandle checks if this file can be handled by the echo extractor
func (e *EchoExtractor) CanHandle(filePath string) bool {
	if filepath.Ext(filePath) != ".go" {
		return false
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	return e.CanHandleContent(filePath, content)
}

// CanHandleContent implements the content-based variant of CanHandle so index
// building can reuse a single read of the file across all extractors.
func (e *EchoExtractor) CanHandleContent(filePath string, content []byte) bool {
	if filepath.Ext(filePath) != ".go" {
		return false
	}

	contentStr := string(content)
	return strings.Contains(contentStr, "github.com/labstack/echo") ||
		strings.Contains(contentStr, "echo.Echo") ||
		strings.Contains(contentStr, "echo.Group")
}

// Extract extracts echo HTTP route contracts from the file
func (e *EchoExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *EchoExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with go/parser. A nil src makes the parser read the
// file from disk; otherwise src is used as the file content.
func (e *EchoExtractor) extract(filePath string, src interface{}) ([]*model.Contract, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var contracts []*model.Contract

	// Find all router method calls: e.GET("/path", handler), e.POST(...), etc.
	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		method := sel.Sel.Name
		if !e.isHTTPMethod(method) {
			return true
		}

		if len(callExpr.Args) < 2 {
			return true
		}

		path := e.extractStringValue(callExpr.Args[0])
		if path == "" {
			return true
		}

		handler := e.extractHandlerName(callExpr.Args[1])

		var line int
		if callExpr.Pos().IsValid() {
			pos := fset.Position(callExpr.Pos())
			line = pos.Line
		}

		signature := map[string]interface{}{
			"method":  method,
			"path":    path,
			"handler": handler,
		}

		contract := &model.Contract{
			Kind:       model.ContractKindHTTPRoute,
			Identifier: fmt.Sprintf("%s %s", method, path),
			File:       filePath,
			Line:       line,
			Signature:  signature,
			Framework:  "echo",
			Language:   model.LanguageGo,
		}

		if err := contract.ComputeSignatureHash(); err == nil {
			contracts = append(contracts, contract)
		}

		return true
	})

	return contracts, nil
}

// IsEntryPoint checks if a symbol is an echo handler entry point
func (e *EchoExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}

	// Echo handlers have signature: func(echo.Context) error
	return strings.Contains(symbol.Signature, "echo.Context")
}

// isHTTPMethod checks if the method name is an HTTP method
func (e *EchoExtractor) isHTTPMethod(method string) bool {
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "Any", "Match", "Add"}
	for _, m := range httpMethods {
		if method == m {
			return true
		}
	}
	return false
}

// extractStringValue extracts string value from an expression
func (e *EchoExtractor) extractStringValue(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s := v.Value
			if len(s) >= 2 {
				s = s[1 : len(s)-1]
			}
			return s
		}
	}
	return ""
}

// extractHandlerName extracts handler name from an expression
func (e *EchoExtractor) extractHandlerName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", e.extractHandlerName(v.X), v.Sel.Name)
	}
	return "anonymous"
}
