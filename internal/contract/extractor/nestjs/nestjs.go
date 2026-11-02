package nestjs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

type NestJSExtractor struct{}

type decoratorRoute struct{ method, path string }

var nestMethods = map[string]string{"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH", "All": "ALL"}
var decoratorRE = regexp.MustCompile(`@([A-Za-z_][A-Za-z0-9_]*)\s*(?:\((?s:(.*))\))?`)

func (e *NestJSExtractor) Name() string             { return "nestjs" }
func (e *NestJSExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *NestJSExtractor) Language() model.Language { return model.LanguageTypeScript }

func (e *NestJSExtractor) CanHandle(filePath string) bool {
	ext := filepath.Ext(filePath)
	if ext != ".ts" && ext != ".tsx" {
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
func (e *NestJSExtractor) CanHandleContent(filePath string, content []byte) bool {
	ext := filepath.Ext(filePath)
	if ext != ".ts" && ext != ".tsx" {
		return false
	}
	return strings.Contains(string(content), "@Controller")
}

func (e *NestJSExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *NestJSExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with tree-sitter. A nil src makes astutil read the
// file from disk; otherwise src is used as the file content.
func (e *NestJSExtractor) extract(filePath string, src []byte) ([]*model.Contract, error) {
	_, tree, content, _, err := astutil.ParseContent(filePath, src)
	if err != nil {
		return nil, err
	}
	// tree is shared and owned by astutil's parse cache; do not Close it
	// here, ClearParseCache releases it after each build/batch.
	var contracts []*model.Contract
	astutil.Walk(tree.RootNode(), func(node *sitter.Node) {
		if node.Kind() != "class_declaration" {
			return
		}
		prefix, ok := controllerPrefix(node, content)
		if !ok {
			return
		}
		body := node.ChildByFieldName("body")
		if body == nil {
			return
		}
		pendingDecorators := []*sitter.Node{}
		for _, child := range astutil.NamedChildren(body) {
			if child.Kind() == "decorator" {
				pendingDecorators = append(pendingDecorators, child)
				continue
			}
			if child.Kind() != "method_definition" {
				pendingDecorators = nil
				continue
			}
			for _, route := range methodDecorators(child, pendingDecorators, content) {
				path := astutil.JoinPath(prefix, route.path)
				identifier := fmt.Sprintf("%s %s", route.method, path)
				contract := &model.Contract{Kind: model.ContractKindHTTPRoute, Identifier: identifier, File: filePath, Line: int(child.StartPosition().Row) + 1, Signature: map[string]interface{}{"method": route.method, "path": path}, Framework: "nestjs", Language: model.LanguageTypeScript}
				if err := contract.ComputeSignatureHash(); err == nil {
					contracts = append(contracts, contract)
				}
			}
			pendingDecorators = nil
		}
	})
	return contracts, nil
}

func controllerPrefix(class *sitter.Node, source []byte) (string, bool) {
	if parent := class.Parent(); parent != nil {
		for _, child := range astutil.NamedChildren(parent) {
			if child.Id() == class.Id() {
				break
			}
			if child.Kind() == "decorator" {
				name, path := parseDecorator(child, source)
				if name == "Controller" {
					return path, true
				}
			}
		}
	}
	for _, child := range astutil.NamedChildren(class) {
		if child.Kind() != "decorator" {
			continue
		}
		name, path := parseDecorator(child, source)
		if name == "Controller" {
			return path, true
		}
	}
	return "", false
}

func methodDecorators(method *sitter.Node, decorators []*sitter.Node, source []byte) []decoratorRoute {
	var routes []decoratorRoute
	for _, child := range decorators {
		name, path := parseDecorator(child, source)
		if method, ok := nestMethods[name]; ok {
			routes = append(routes, decoratorRoute{method: method, path: path})
		}
	}
	for _, child := range astutil.NamedChildren(method) {
		if child.Kind() != "decorator" {
			continue
		}
		name, path := parseDecorator(child, source)
		if method, ok := nestMethods[name]; ok {
			routes = append(routes, decoratorRoute{method: method, path: path})
		}
	}
	return routes
}

func parseDecorator(decorator *sitter.Node, source []byte) (string, string) {
	text := astutil.Text(decorator, source)
	match := decoratorRE.FindStringSubmatch(text)
	if match == nil {
		return "", ""
	}
	path := ""
	if len(match) > 2 {
		arg := strings.TrimSpace(match[2])
		if len(arg) >= 2 {
			quote := arg[0]
			if quote == '\'' || quote == '"' || quote == '`' {
				if end := strings.LastIndexByte(arg[1:], quote); end >= 0 {
					path = arg[1 : end+1]
				}
			}
		}
	}
	return match[1], path
}

func (e *NestJSExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	return symbol != nil && (strings.Contains(strings.ToLower(symbol.Name), "controller") || strings.Contains(strings.ToLower(symbol.Package), "controller"))
}
