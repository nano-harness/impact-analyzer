package cobra

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

// CobraExtractor extracts CLI command contracts from spf13/cobra-based applications
type CobraExtractor struct {
	ssaExtractor *SSAExtractor
}

// NewCobraExtractor creates a new cobra extractor with optional SSA support
func NewCobraExtractor() *CobraExtractor {
	return &CobraExtractor{
		ssaExtractor: NewSSAExtractor(),
	}
}

// EnableSSA enables SSA-based extraction for advanced patterns
func (e *CobraExtractor) EnableSSA(enabled bool) {
	e.ssaExtractor.SetEnabled(enabled)
}

// Name returns the extractor name
func (e *CobraExtractor) Name() string {
	return "cobra"
}

// Kind returns the contract kind
func (e *CobraExtractor) Kind() model.ContractKind {
	return model.ContractKindCLICommand
}

// Language returns the supported language
func (e *CobraExtractor) Language() model.Language {
	return model.LanguageGo
}

// CanHandle checks if this file can be handled by the cobra extractor
func (e *CobraExtractor) CanHandle(filePath string) bool {
	// Check file extension
	if filepath.Ext(filePath) != ".go" {
		return false
	}

	// Quick heuristic: check if file imports cobra
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	return e.CanHandleContent(filePath, content)
}

// CanHandleContent implements the content-based variant of CanHandle so index
// building can reuse a single read of the file across all extractors.
func (e *CobraExtractor) CanHandleContent(filePath string, content []byte) bool {
	// Check file extension
	if filepath.Ext(filePath) != ".go" {
		return false
	}

	contentStr := string(content)
	return strings.Contains(contentStr, "github.com/spf13/cobra") ||
		strings.Contains(contentStr, "cobra.Command")
}

// Extract extracts cobra command contracts from the file
func (e *CobraExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *CobraExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract runs the SSA-first extraction flow. src is only used by the AST
// fallback: a nil src makes go/parser read the file from disk.
func (e *CobraExtractor) extract(filePath string, src interface{}) ([]*model.Contract, error) {
	// Try SSA extraction first if enabled (defensive: check for nil)
	if e.ssaExtractor != nil && e.ssaExtractor.IsEnabled() {
		// Determine package path from file path
		pkgPath := filepath.Dir(filePath)
		contracts, err := e.ssaExtractor.ExtractFromPackage(pkgPath)
		if err == nil && len(contracts) > 0 {
			return contracts, nil
		}
		if debugEnabled() {
			if err != nil {
				fmt.Fprintf(os.Stderr, "[cobra] SSA failed (%v), falling back to AST\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "[cobra] SSA returned 0 contracts, falling back to AST\n")
			}
		}
	}

	return e.extractWithAST(filePath, src)
}

func (e *CobraExtractor) extractWithAST(filePath string, src interface{}) ([]*model.Contract, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	fileFlags := e.extractFlagsFromASTFile(node)
	var contracts []*model.Contract

	// Find all &cobra.Command{...} composite literals
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for &cobra.Command{...} or cobra.Command{...}
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Check if this is a cobra.Command type
		if !e.isCobraCommand(compLit.Type) {
			return true
		}

		// Extract command contract
		contract := e.extractCommand(compLit, filePath, fset)
		if contract != nil {
			if len(fileFlags) > 0 {
				contract.Signature["flags"] = fileFlags
				if err := contract.ComputeSignatureHash(); err != nil {
					if debugEnabled() {
						fmt.Fprintf(os.Stderr, "[cobra] Failed to compute hash for %s: %v\n", contract.Identifier, err)
					}
					return true
				}
			}
			contracts = append(contracts, contract)
		}

		return true
	})

	return contracts, nil
}

// extractFlagsFromASTFile scans file-level cobra flag binding calls.
func (e *CobraExtractor) extractFlagsFromASTFile(node *ast.File) []map[string]interface{} {
	var flags []map[string]interface{}
	seen := make(map[string]bool)

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		flagType := classifyASTFlagBindCall(sel.Sel.Name)
		if flagType == "" || !isASTFlagsReceiver(sel.X) || len(call.Args) < 2 {
			return true
		}

		flagName := e.extractStringValue(call.Args[1])
		if flagName == "" || seen[flagName] {
			return true
		}

		seen[flagName] = true
		flags = append(flags, map[string]interface{}{
			"name": flagName,
			"type": flagType,
		})
		return true
	})

	return flags
}

func classifyASTFlagBindCall(methodName string) string {
	return flagBindingMethods[methodName]
}

func isASTFlagsReceiver(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	return sel.Sel.Name == "Flags" || sel.Sel.Name == "PersistentFlags"
}

// IsEntryPoint checks if a symbol is a cobra command entry point
func (e *CobraExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}

	// Rule 1: main.Execute (only in main package, exact name)
	if symbol.Name == "Execute" && symbol.Package == "main" {
		return true
	}

	// Rule 2: rootCmd variable (the standard cobra root command)
	if symbol.Name == "rootCmd" && symbol.Kind == model.SymbolKindVariable {
		return true
	}

	// Note: Rule 3 (SSA-based factory function detection) has been removed.
	// With SSA instance isolation (Bug #1 fix), we no longer maintain a persistent
	// prog field, so we cannot perform cross-package SSA signature verification here.
	// Factory functions returning *cobra.Command are now only detected during
	// ExtractFromPackage calls when SSA is enabled.

	return false
}

// isCobraCommand checks if the type is cobra.Command
func (e *CobraExtractor) isCobraCommand(typ ast.Expr) bool {
	switch t := typ.(type) {
	case *ast.SelectorExpr:
		// cobra.Command
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "cobra" && t.Sel.Name == "Command"
		}
	case *ast.UnaryExpr:
		// &cobra.Command
		if t.Op == token.AND {
			return e.isCobraCommand(t.X)
		}
	case *ast.StarExpr:
		// *cobra.Command
		return e.isCobraCommand(t.X)
	}
	return false
}

// extractCommand extracts a cobra command contract
func (e *CobraExtractor) extractCommand(compLit *ast.CompositeLit, filePath string, fset *token.FileSet) *model.Contract {
	signature := make(map[string]interface{})
	var line int

	// Get position
	if compLit.Pos().IsValid() {
		pos := fset.Position(compLit.Pos())
		line = pos.Line
	}

	// Extract fields from composite literal
	var use, short, long string
	var aliases []string

	for _, elt := range compLit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "Use":
			use = e.extractStringValue(kv.Value)
		case "Short":
			short = e.extractStringValue(kv.Value)
		case "Long":
			long = e.extractStringValue(kv.Value)
		case "Aliases":
			aliases = e.extractStringSlice(kv.Value)
		}
	}

	// Build signature
	signature["use"] = use
	if short != "" {
		signature["short"] = short
	}
	if long != "" {
		signature["long"] = long
	}
	if len(aliases) > 0 {
		signature["aliases"] = aliases
	}

	// For now, we don't extract flags from command definitions as they are typically
	// added via separate Flags() calls which would require control flow analysis
	// This simplified version focuses on command metadata

	// Build identifier from use
	identifier := use
	if identifier == "" {
		identifier = fmt.Sprintf("command-at-line-%d", line)
	}

	contract := &model.Contract{
		Kind:       model.ContractKindCLICommand,
		Identifier: identifier,
		File:       filePath,
		Line:       line,
		Signature:  signature,
		Framework:  "cobra",
		Language:   model.LanguageGo,
	}

	// Compute signature hash
	if err := contract.ComputeSignatureHash(); err != nil {
		return nil
	}

	return contract
}

// extractStringValue extracts string value from an expression
func (e *CobraExtractor) extractStringValue(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			// Remove quotes
			s := v.Value
			if len(s) >= 2 {
				s = s[1 : len(s)-1]
			}
			return s
		}
	}
	return ""
}

// extractStringSlice extracts string slice from an expression
func (e *CobraExtractor) extractStringSlice(expr ast.Expr) []string {
	compLit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}

	var result []string
	for _, elt := range compLit.Elts {
		if s := e.extractStringValue(elt); s != "" {
			result = append(result, s)
		}
	}
	return result
}
