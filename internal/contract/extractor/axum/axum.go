package axum

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

type AxumExtractor struct{}

type rustStep struct {
	method string
	args   *sitter.Node
	line   int
}
type axumRoute struct {
	method, path, handler string
	line                  int
}
type axumNest struct {
	prefix, binding string
	line            int
}
type routerValue struct {
	routes []axumRoute
	nests  []axumNest
}

var axumMethods = map[string]string{"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE", "patch": "PATCH", "head": "HEAD", "options": "OPTIONS", "trace": "TRACE"}

func (e *AxumExtractor) Name() string             { return "axum" }
func (e *AxumExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *AxumExtractor) Language() model.Language { return model.LanguageRust }

func (e *AxumExtractor) CanHandle(filePath string) bool {
	if filepath.Ext(filePath) != ".rs" {
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
func (e *AxumExtractor) CanHandleContent(filePath string, content []byte) bool {
	if filepath.Ext(filePath) != ".rs" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "Router::new") || strings.Contains(text, "axum")
}

func (e *AxumExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *AxumExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with tree-sitter. A nil src makes astutil read the
// file from disk; otherwise src is used as the file content.
func (e *AxumExtractor) extract(filePath string, src []byte) ([]*model.Contract, error) {
	_, tree, content, _, err := astutil.ParseContent(filePath, src)
	if err != nil {
		return nil, err
	}
	// tree is shared and owned by astutil's parse cache; do not Close it
	// here, ClearParseCache releases it after each build/batch.
	bindings := map[string]routerValue{}
	nested := map[string]bool{}
	var anonymous []axumRoute
	astutil.Walk(tree.RootNode(), func(node *sitter.Node) {
		if node.Kind() != "let_declaration" {
			return
		}
		pattern := node.ChildByFieldName("pattern")
		value := node.ChildByFieldName("value")
		if pattern == nil || value == nil || pattern.Kind() != "identifier" {
			return
		}
		rv := parseRouterValue(value, content)
		if len(rv.routes) > 0 || len(rv.nests) > 0 {
			bindings[astutil.Text(pattern, content)] = rv
			for _, nest := range rv.nests {
				nested[nest.binding] = true
			}
		}
	})
	var topRoutes []axumRoute
	for name, rv := range bindings {
		if !nested[name] {
			topRoutes = append(topRoutes, resolveRouter(rv, bindings, map[string]bool{})...)
		}
	}
	astutil.Walk(tree.RootNode(), func(node *sitter.Node) {
		if node.Kind() != "call_expression" || !astutil.IsOutermostCall(node) {
			return
		}
		if parent := enclosingLet(node); parent != nil {
			return
		}
		rv := parseRouterValue(node, content)
		topRoutes = append(topRoutes, resolveRouter(rv, bindings, map[string]bool{})...)
		for _, nest := range rv.nests {
			if _, ok := bindings[nest.binding]; !ok {
				anonymous = append(anonymous, axumRoute{method: "ANY", path: astutil.JoinPath(nest.prefix, ""), handler: "anonymous", line: nest.line})
			}
		}
	})
	topRoutes = append(topRoutes, anonymous...)
	seen := map[string]bool{}
	var contracts []*model.Contract
	for _, r := range topRoutes {
		id := fmt.Sprintf("%s %s", r.method, r.path)
		if seen[id] {
			continue
		}
		seen[id] = true
		contract := &model.Contract{Kind: model.ContractKindHTTPRoute, Identifier: id, File: filePath, Line: r.line, Signature: map[string]interface{}{"method": r.method, "path": r.path, "handler": r.handler}, Framework: "axum", Language: model.LanguageRust}
		if err := contract.ComputeSignatureHash(); err == nil {
			contracts = append(contracts, contract)
		}
	}
	return contracts, nil
}

func parseRouterValue(node *sitter.Node, source []byte) routerValue {
	_, steps := collectRustChain(node, source)
	var rv routerValue
	for _, step := range steps {
		switch step.method {
		case "route":
			path, ok := astutil.StringValue(astutil.ArgNode(step.args, 0), source)
			if !ok {
				continue
			}
			rv.routes = append(rv.routes, extractHandlers(path, astutil.ArgNode(step.args, 1), step.line, source)...)
		case "nest":
			prefix, ok := astutil.StringValue(astutil.ArgNode(step.args, 0), source)
			bindingNode := astutil.ArgNode(step.args, 1)
			if ok && bindingNode != nil && bindingNode.Kind() == "identifier" {
				rv.nests = append(rv.nests, axumNest{prefix: prefix, binding: astutil.Text(bindingNode, source), line: step.line})
			}
		}
	}
	return rv
}

func collectRustChain(call *sitter.Node, source []byte) (string, []rustStep) {
	var steps []rustStep
	current := call
	for current != nil && current.Kind() == "call_expression" {
		fn := current.ChildByFieldName("function")
		args := current.ChildByFieldName("arguments")
		if fn == nil {
			return "", steps
		}
		if fn.Kind() == "field_expression" {
			field := fn.ChildByFieldName("field")
			value := fn.ChildByFieldName("value")
			if field == nil || value == nil {
				break
			}
			steps = append([]rustStep{{method: astutil.Text(field, source), args: args, line: int(field.StartPosition().Row) + 1}}, steps...)
			if value.Kind() == "call_expression" {
				current = value
				continue
			}
			return astutil.Text(value, source), steps
		}
		return astutil.Text(fn, source), steps
	}
	return "", steps
}

func extractHandlers(path string, node *sitter.Node, line int, source []byte) []axumRoute {
	var routes []axumRoute
	if node == nil {
		return routes
	}
	if node.Kind() != "call_expression" {
		return routes
	}
	fn := node.ChildByFieldName("function")
	args := node.ChildByFieldName("arguments")
	if fn == nil {
		return routes
	}
	switch fn.Kind() {
	case "identifier":
		name := astutil.Text(fn, source)
		if method, ok := axumMethods[name]; ok {
			routes = append(routes, axumRoute{method: method, path: astutil.JoinPath("", path), handler: rustHandlerName(astutil.ArgNode(args, 0), source), line: int(fn.StartPosition().Row) + 1})
		}
	case "field_expression":
		field := fn.ChildByFieldName("field")
		value := fn.ChildByFieldName("value")
		if value != nil {
			routes = append(routes, extractHandlers(path, value, line, source)...)
		}
		if field != nil {
			name := astutil.Text(field, source)
			if method, ok := axumMethods[name]; ok {
				routes = append(routes, axumRoute{method: method, path: astutil.JoinPath("", path), handler: rustHandlerName(astutil.ArgNode(args, 0), source), line: int(field.StartPosition().Row) + 1})
			}
		}
	}
	return routes
}

func resolveRouter(rv routerValue, bindings map[string]routerValue, visiting map[string]bool) []axumRoute {
	routes := append([]axumRoute{}, rv.routes...)
	for _, nest := range rv.nests {
		child, ok := bindings[nest.binding]
		if !ok || visiting[nest.binding] {
			continue
		}
		visiting[nest.binding] = true
		for _, route := range resolveRouter(child, bindings, visiting) {
			route.path = astutil.JoinPath(nest.prefix, route.path)
			routes = append(routes, route)
		}
		visiting[nest.binding] = false
	}
	return routes
}

func enclosingLet(node *sitter.Node) *sitter.Node {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == "let_declaration" {
			return p
		}
	}
	return nil
}

func rustHandlerName(node *sitter.Node, source []byte) string {
	if node == nil {
		return "anonymous"
	}
	if node.Kind() == "identifier" || node.Kind() == "scoped_identifier" {
		return astutil.Text(node, source)
	}
	return "anonymous"
}

func (e *AxumExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	return symbol != nil && strings.HasSuffix(symbol.Name, "handler")
}
