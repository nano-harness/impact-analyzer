package express

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ExpressExtractor extracts HTTP route contracts from Express.js applications.
type ExpressExtractor struct{}

type callStep struct {
	method string
	args   *sitter.Node
	line   int
}

type routeInfo struct {
	receiver string
	method   string
	path     string
	handler  string
	line     int
}

type mount struct{ parent, child, prefix string }

var httpMethods = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE", "patch": "PATCH",
	"head": "HEAD", "options": "OPTIONS", "all": "ALL",
}

func (e *ExpressExtractor) Name() string             { return "express" }
func (e *ExpressExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *ExpressExtractor) Language() model.Language { return model.LanguageJavaScript }

func (e *ExpressExtractor) CanHandle(filePath string) bool {
	ext := filepath.Ext(filePath)
	if ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" {
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
func (e *ExpressExtractor) CanHandleContent(filePath string, content []byte) bool {
	ext := filepath.Ext(filePath)
	if ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "express") || strings.Contains(text, ".Router()") || strings.Contains(text, ".route(")
}

func (e *ExpressExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *ExpressExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with tree-sitter. A nil src makes astutil read the
// file from disk; otherwise src is used as the file content.
func (e *ExpressExtractor) extract(filePath string, src []byte) ([]*model.Contract, error) {
	_, tree, content, lang, err := astutil.ParseContent(filePath, src)
	if err != nil {
		return nil, err
	}
	// tree is shared and owned by astutil's parse cache; do not Close it
	// here, ClearParseCache releases it after each build/batch.

	routers := map[string]bool{}
	var mounts []mount
	var routes []routeInfo

	astutil.Walk(tree.RootNode(), func(node *sitter.Node) {
		switch node.Kind() {
		case "variable_declarator":
			name := node.ChildByFieldName("name")
			value := node.ChildByFieldName("value")
			if name != nil && value != nil && strings.Contains(astutil.Text(value, content), ".Router(") {
				routers[astutil.Text(name, content)] = true
			}
		case "assignment_expression":
			left := node.ChildByFieldName("left")
			right := node.ChildByFieldName("right")
			if left != nil && right != nil && strings.Contains(astutil.Text(right, content), ".Router(") {
				routers[astutil.Text(left, content)] = true
			}
		case "call_expression":
			if !astutil.IsOutermostCall(node) {
				return
			}
			base, steps := collectJSChain(node, content)
			if len(steps) == 0 {
				return
			}
			if len(steps) == 1 && steps[0].method == "use" {
				if prefix, child, ok := parseUse(steps[0].args, content); ok {
					mounts = append(mounts, mount{parent: base, child: child, prefix: prefix})
					routers[child] = true
				}
			}
			if len(steps) == 1 {
				if method, ok := httpMethods[steps[0].method]; ok {
					if path, ok := astutil.FirstStringArg(steps[0].args, content); ok {
						routes = append(routes, routeInfo{receiver: base, method: method, path: path, handler: handlerName(astutil.ArgNode(steps[0].args, 1), content), line: steps[0].line})
					}
				}
			}
			for i, step := range steps {
				if step.method != "route" {
					continue
				}
				path, ok := astutil.FirstStringArg(step.args, content)
				if !ok {
					continue
				}
				for _, next := range steps[i+1:] {
					if method, ok := httpMethods[next.method]; ok {
						routes = append(routes, routeInfo{receiver: base, method: method, path: path, handler: handlerName(astutil.ArgNode(next.args, 0), content), line: next.line})
					}
				}
			}
		}
	})

	prefixes := computeRouterPrefixes(mounts, routers)
	var contracts []*model.Contract
	seen := map[string]bool{}
	for _, r := range routes {
		prefixList := []string{""}
		if routers[r.receiver] && len(prefixes[r.receiver]) > 0 {
			prefixList = prefixes[r.receiver]
		}
		for _, prefix := range prefixList {
			path := astutil.JoinPath(prefix, r.path)
			key := r.method + " " + path
			if seen[key] {
				continue
			}
			seen[key] = true
			contract := &model.Contract{Kind: model.ContractKindHTTPRoute, Identifier: key, File: filePath, Line: r.line, Signature: map[string]interface{}{"method": r.method, "path": path, "handler": cleanHandler(r.handler)}, Framework: "express", Language: lang}
			if err := contract.ComputeSignatureHash(); err == nil {
				contracts = append(contracts, contract)
			}
		}
	}
	return contracts, nil
}

func collectJSChain(call *sitter.Node, source []byte) (string, []callStep) {
	var steps []callStep
	current := call
	for current != nil && current.Kind() == "call_expression" {
		fn := current.ChildByFieldName("function")
		args := current.ChildByFieldName("arguments")
		if fn == nil {
			return astutil.Text(current, source), steps
		}
		if fn.Kind() == "member_expression" {
			prop := fn.ChildByFieldName("property")
			obj := fn.ChildByFieldName("object")
			if prop == nil || obj == nil {
				break
			}
			steps = append([]callStep{{method: astutil.Text(prop, source), args: args, line: int(prop.StartPosition().Row) + 1}}, steps...)
			if obj.Kind() == "call_expression" {
				current = obj
				continue
			}
			return astutil.Text(obj, source), steps
		}
		return astutil.Text(fn, source), steps
	}
	return "", steps
}

func parseUse(args *sitter.Node, source []byte) (string, string, bool) {
	prefix, ok := astutil.StringValue(astutil.ArgNode(args, 0), source)
	if !ok {
		return "", "", false
	}
	child := astutil.ArgNode(args, 1)
	if child == nil || child.Kind() != "identifier" {
		return "", "", false
	}
	return prefix, astutil.Text(child, source), true
}

func computeRouterPrefixes(mounts []mount, routers map[string]bool) map[string][]string {
	prefixes := map[string][]string{}
	for _, m := range mounts {
		if !routers[m.parent] {
			prefixes[m.child] = append(prefixes[m.child], astutil.JoinPath("", m.prefix))
		}
	}
	changed := true
	for changed {
		changed = false
		for _, m := range mounts {
			for _, parentPrefix := range prefixes[m.parent] {
				next := astutil.JoinPath(parentPrefix, m.prefix)
				if !contains(prefixes[m.child], next) {
					prefixes[m.child] = append(prefixes[m.child], next)
					changed = true
				}
			}
		}
	}
	return prefixes
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func handlerName(node *sitter.Node, source []byte) string {
	if node == nil {
		return "anonymous"
	}
	switch node.Kind() {
	case "identifier", "member_expression":
		return astutil.Text(node, source)
	default:
		if strings.Contains(node.Kind(), "function") {
			return "anonymous"
		}
		return "anonymous"
	}
}

func cleanHandler(handler string) string {
	handler = strings.TrimPrefix(handler, ".")
	parts := strings.SplitN(handler, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return handler
}

func (e *ExpressExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}
	sig := strings.ToLower(symbol.Signature)
	return strings.Contains(sig, "req") && strings.Contains(sig, "res")
}
