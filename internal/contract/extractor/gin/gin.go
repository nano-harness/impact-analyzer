package gin

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

// GinExtractor extracts HTTP route contracts from gin-gonic/gin applications
type GinExtractor struct{}

// Name returns the extractor name
func (e *GinExtractor) Name() string {
	return "gin"
}

// Kind returns the contract kind
func (e *GinExtractor) Kind() model.ContractKind {
	return model.ContractKindHTTPRoute
}

// Language returns the supported language
func (e *GinExtractor) Language() model.Language {
	return model.LanguageGo
}

// CanHandle checks if this file can be handled by the gin extractor
func (e *GinExtractor) CanHandle(filePath string) bool {
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
func (e *GinExtractor) CanHandleContent(filePath string, content []byte) bool {
	if filepath.Ext(filePath) != ".go" {
		return false
	}

	contentStr := string(content)
	return strings.Contains(contentStr, "github.com/gin-gonic/gin") ||
		strings.Contains(contentStr, "gin.Engine") ||
		strings.Contains(contentStr, "gin.RouterGroup")
}

// Extract extracts gin HTTP route contracts from the file
func (e *GinExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *GinExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with go/parser. A nil src makes the parser read the
// file from disk; otherwise src is used as the file content.
func (e *GinExtractor) extract(filePath string, src interface{}) ([]*model.Contract, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var contracts []*model.Contract

	// Find all router method calls: r.GET("/path", handler), r.POST(...), etc.
	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if this is a router method call
		sel, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		method := sel.Sel.Name
		if !e.isHTTPMethod(method) {
			return true
		}

		// Extract path and handler
		if len(callExpr.Args) < 2 {
			return true
		}

		path := e.extractStringValue(callExpr.Args[0])
		if path == "" {
			return true
		}

		handler := e.extractHandlerName(callExpr.Args[1])

		// Get line number
		var line int
		if callExpr.Pos().IsValid() {
			pos := fset.Position(callExpr.Pos())
			line = pos.Line
		}

		// Build contract
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
			Framework:  "gin",
			Language:   model.LanguageGo,
		}

		if err := contract.ComputeSignatureHash(); err == nil {
			contracts = append(contracts, contract)
		}

		return true
	})

	return contracts, nil
}

// IsEntryPoint checks if a symbol is a gin handler entry point
func (e *GinExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}

	// Gin handlers have signature: func(*gin.Context)
	// Check if signature contains gin.Context
	return strings.Contains(symbol.Signature, "gin.Context")
}

// isHTTPMethod checks if the method name is an HTTP method
func (e *GinExtractor) isHTTPMethod(method string) bool {
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "Any", "Handle"}
	for _, m := range httpMethods {
		if method == m {
			return true
		}
	}
	return false
}

// extractStringValue extracts string value from an expression
func (e *GinExtractor) extractStringValue(expr ast.Expr) string {
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
func (e *GinExtractor) extractHandlerName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", e.extractHandlerName(v.X), v.Sel.Name)
	}
	return "anonymous"
}
