package springmvc

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

type SpringMVCExtractor struct{}

type javaMapping struct {
	method string
	paths  []string
}

type javaParam struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

var springMethods = map[string]string{"GetMapping": "GET", "PostMapping": "POST", "PutMapping": "PUT", "DeleteMapping": "DELETE", "PatchMapping": "PATCH"}
var requestMethodRE = regexp.MustCompile(`RequestMethod\.([A-Z]+)`)

func (e *SpringMVCExtractor) Name() string             { return "spring-mvc" }
func (e *SpringMVCExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *SpringMVCExtractor) Language() model.Language { return model.LanguageJava }

func (e *SpringMVCExtractor) CanHandle(filePath string) bool {
	if filepath.Ext(filePath) != ".java" {
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
func (e *SpringMVCExtractor) CanHandleContent(filePath string, content []byte) bool {
	if filepath.Ext(filePath) != ".java" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "@RestController") ||
		strings.Contains(text, "@Controller") ||
		strings.Contains(text, "@RequestMapping") ||
		strings.Contains(text, "@GetMapping") ||
		strings.Contains(text, "@PostMapping")
}

func (e *SpringMVCExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *SpringMVCExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with tree-sitter. A nil src makes astutil read the
// file from disk; otherwise src is used as the file content.
func (e *SpringMVCExtractor) extract(filePath string, src []byte) ([]*model.Contract, error) {
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
		classAnnotations := javaAnnotations(javaModifiers(node), content)
		if !isController(classAnnotations, content) {
			return
		}
		className := javaNodeName(node, content)
		classPaths := []string{""}
		for _, ann := range classAnnotations {
			if annotationName(ann, content) == "RequestMapping" {
				if paths := annotationPaths(ann, content); len(paths) > 0 {
					classPaths = paths
				}
			}
		}
		body := node.ChildByFieldName("body")
		if body == nil {
			return
		}
		for _, child := range astutil.NamedChildren(body) {
			if child.Kind() != "method_declaration" {
				continue
			}
			for _, mapping := range methodMappings(javaAnnotations(javaModifiers(child), content), content) {
				for _, cp := range classPaths {
					for _, mp := range mapping.paths {
						path := astutil.JoinPath(cp, mp)
						method := mapping.method
						methodName := javaNodeName(child, content)
						handler := methodName
						if className != "" && methodName != "" {
							handler = className + "." + methodName
						}
						contract := &model.Contract{Kind: model.ContractKindHTTPRoute, Identifier: fmt.Sprintf("%s %s", method, path), File: filePath, Line: int(child.StartPosition().Row) + 1, Signature: map[string]interface{}{"method": method, "path": path, "params": extractParams(child, content), "response": extractReturnType(child, content), "handler": handler}, Framework: "spring-mvc", Language: model.LanguageJava}
						if err := contract.ComputeSignatureHash(); err == nil {
							contracts = append(contracts, contract)
						}
					}
				}
			}
		}
	})
	return contracts, nil
}

func javaAnnotations(mods *sitter.Node, source []byte) []*sitter.Node {
	var out []*sitter.Node
	if mods == nil {
		return out
	}
	for _, child := range astutil.NamedChildren(mods) {
		if child.Kind() == "annotation" || child.Kind() == "marker_annotation" {
			out = append(out, child)
		}
	}
	return out
}

func javaModifiers(node *sitter.Node) *sitter.Node {
	if mods := node.ChildByFieldName("modifiers"); mods != nil {
		return mods
	}
	for _, child := range astutil.NamedChildren(node) {
		if child.Kind() == "modifiers" {
			return child
		}
	}
	return nil
}

func javaNodeName(node *sitter.Node, source []byte) string {
	if name := node.ChildByFieldName("name"); name != nil {
		return astutil.Text(name, source)
	}
	return ""
}

func annotationName(ann *sitter.Node, source []byte) string {
	name := ann.ChildByFieldName("name")
	if name == nil && ann.NamedChildCount() > 0 {
		name = ann.NamedChild(0)
	}
	return astutil.Text(name, source)
}

func isController(annotations []*sitter.Node, source []byte) bool {
	for _, ann := range annotations {
		name := annotationName(ann, source)
		if name == "RestController" || name == "Controller" {
			return true
		}
	}
	return false
}

func methodMappings(annotations []*sitter.Node, source []byte) []javaMapping {
	var out []javaMapping
	for _, ann := range annotations {
		name := annotationName(ann, source)
		paths := annotationPaths(ann, source)
		if len(paths) == 0 {
			paths = []string{""}
		}
		if method, ok := springMethods[name]; ok {
			out = append(out, javaMapping{method: method, paths: paths})
			continue
		}
		if name == "RequestMapping" {
			methods := requestMethods(ann, source)
			if len(methods) == 0 {
				methods = []string{"REQUEST"}
			}
			for _, method := range methods {
				out = append(out, javaMapping{method: method, paths: paths})
			}
		}
	}
	return out
}

func annotationPaths(ann *sitter.Node, source []byte) []string {
	paths := astutil.StringLiterals(ann, source)
	if len(paths) == 0 {
		return nil
	}
	return paths
}

func requestMethods(ann *sitter.Node, source []byte) []string {
	matches := requestMethodRE.FindAllStringSubmatch(astutil.Text(ann, source), -1)
	var methods []string
	for _, match := range matches {
		methods = append(methods, match[1])
	}
	return methods
}

func extractParams(method *sitter.Node, source []byte) []map[string]interface{} {
	paramsNode := method.ChildByFieldName("parameters")
	if paramsNode == nil {
		return []map[string]interface{}{}
	}
	params := []map[string]interface{}{}
	for _, child := range astutil.NamedChildren(paramsNode) {
		if !strings.Contains(child.Kind(), "parameter") {
			continue
		}
		text := astutil.Text(child, source)
		name := javaNodeName(child, source)
		if name == "" {
			fields := strings.Fields(stripJavaAnnotations(text))
			if len(fields) > 0 {
				name = strings.Trim(fields[len(fields)-1], "...")
			}
		}
		typ := javaParamType(child, source, name)
		params = append(params, map[string]interface{}{"name": name, "type": typ, "source": javaParamSource(text)})
	}
	return params
}

func extractReturnType(method *sitter.Node, source []byte) string {
	if typ := method.ChildByFieldName("type"); typ != nil {
		return strings.TrimSpace(astutil.Text(typ, source))
	}
	text := stripJavaAnnotations(astutil.Text(method, source))
	beforeName := text
	if name := javaNodeName(method, source); name != "" {
		if idx := strings.Index(beforeName, name+"("); idx >= 0 {
			beforeName = beforeName[:idx]
		}
	}
	fields := strings.Fields(beforeName)
	if len(fields) == 0 {
		return "void"
	}
	return fields[len(fields)-1]
}

func javaParamType(param *sitter.Node, source []byte, name string) string {
	if typ := param.ChildByFieldName("type"); typ != nil {
		return strings.TrimSpace(astutil.Text(typ, source))
	}
	text := stripJavaAnnotations(astutil.Text(param, source))
	if name != "" {
		text = strings.TrimSpace(strings.TrimSuffix(text, name))
	}
	return strings.TrimSpace(text)
}

func javaParamSource(text string) string {
	switch {
	case strings.Contains(text, "@PathVariable"):
		return "path"
	case strings.Contains(text, "@RequestParam"):
		return "query"
	case strings.Contains(text, "@RequestBody"):
		return "body"
	case strings.Contains(text, "@RequestHeader"):
		return "header"
	default:
		return "unknown"
	}
}

func stripJavaAnnotations(text string) string {
	re := regexp.MustCompile(`@[A-Za-z0-9_.]+(\([^)]*\))?`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}

func (e *SpringMVCExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}
	return strings.Contains(symbol.Package, "controller") || strings.Contains(strings.ToLower(symbol.Name), "controller")
}
