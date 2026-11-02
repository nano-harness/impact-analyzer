package reactrouter

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type ReactRouterExtractor struct{}

type frontendRoute struct {
	path, component string
	exact, lazy     bool
	line            int
}

var pathPropRE = regexp.MustCompile(`\bpath\s*:\s*["']([^"']*)["']`)
var componentPropRE = regexp.MustCompile(`\b(?:component|element)\s*:\s*([^,}\n]+)`)
var jsxRouteRE = regexp.MustCompile(`(?s)<Route\b[^>]*\bpath\s*=\s*["']([^"']*)["'][^>]*>`)
var jsxComponentRE = regexp.MustCompile(`\b(?:component|element)\s*=\s*(?:\{?([A-Za-z_][A-Za-z0-9_\.]*)\}?|\{\s*<([A-Za-z_][A-Za-z0-9_\.]*)[^}]*\})`)

func (e *ReactRouterExtractor) Name() string             { return "react-router" }
func (e *ReactRouterExtractor) Kind() model.ContractKind { return model.ContractKindFrontendRoute }
func (e *ReactRouterExtractor) Language() model.Language { return model.LanguageJavaScript }

func (e *ReactRouterExtractor) CanHandle(filePath string) bool {
	ext := filepath.Ext(filePath)
	if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
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
func (e *ReactRouterExtractor) CanHandleContent(filePath string, content []byte) bool {
	ext := filepath.Ext(filePath)
	if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "IRouterConfig") || strings.Contains(text, "createBrowserRouter") || strings.Contains(text, "createHashRouter") || strings.Contains(text, "<Route")
}

func (e *ReactRouterExtractor) Extract(filePath string) ([]*model.Contract, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return e.ExtractContent(filePath, content)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *ReactRouterExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	text := string(content)
	routes := extractObjectRoutes(text)
	routes = append(routes, extractJSXRoutes(text)...)
	seen := map[string]bool{}
	var contracts []*model.Contract
	lang := astutil.LanguageForPath(filePath)
	for _, r := range routes {
		if r.path == "" {
			continue
		}
		id := "FRONTEND " + normalizeRoutePath(r.path)
		if seen[id] {
			continue
		}
		seen[id] = true
		sig := map[string]interface{}{"path": normalizeRoutePath(r.path), "component": r.component, "exact": r.exact, "lazy": r.lazy}
		contract := &model.Contract{Kind: model.ContractKindFrontendRoute, Identifier: id, File: filePath, Line: r.line, Signature: sig, Framework: "react-router", Language: lang}
		if err := contract.ComputeSignatureHash(); err == nil {
			contracts = append(contracts, contract)
		}
	}
	return contracts, nil
}

func extractObjectRoutes(text string) []frontendRoute {
	matches := pathPropRE.FindAllStringSubmatchIndex(text, -1)
	routes := make([]frontendRoute, 0, len(matches))
	for _, m := range matches {
		path := text[m[2]:m[3]]
		start, end := enclosingObject(text, m[0])
		if start < 0 || end < 0 {
			continue
		}
		objectText := text[start : end+1]
		parent := parentRoutePath(text, start)
		fullPath := normalizeNestedPath(parent, path)
		routes = append(routes, frontendRoute{path: fullPath, component: routeComponent(objectText), exact: strings.Contains(objectText, "exact"), lazy: strings.Contains(objectText, "lazy("), line: 1 + strings.Count(text[:m[0]], "\n")})
	}
	return routes
}

func extractJSXRoutes(text string) []frontendRoute {
	matches := jsxRouteRE.FindAllStringSubmatchIndex(text, -1)
	var routes []frontendRoute
	for _, m := range matches {
		tag := text[m[0]:m[1]]
		component := ""
		if cm := jsxComponentRE.FindStringSubmatch(tag); len(cm) > 0 {
			if cm[1] != "" {
				component = cm[1]
			} else {
				component = cm[2]
			}
		}
		routes = append(routes, frontendRoute{path: text[m[2]:m[3]], component: component, exact: strings.Contains(tag, " exact") || strings.Contains(tag, " exact="), lazy: strings.Contains(tag, "lazy("), line: 1 + strings.Count(text[:m[0]], "\n")})
	}
	return routes
}

func enclosingObject(text string, pos int) (int, int) {
	start := -1
	depth := 0
	for i := pos; i >= 0; i-- {
		switch text[i] {
		case '}':
			depth++
		case '{':
			if depth == 0 {
				start = i
				i = -1
			} else {
				depth--
			}
		}
	}
	if start < 0 {
		return -1, -1
	}
	end := matching(text, start, '{', '}')
	return start, end
}

func parentRoutePath(text string, objectStart int) string {
	for search := objectStart - 1; search >= 0; {
		start, end := enclosingObject(text, search)
		if start < 0 || end < objectStart {
			return ""
		}
		obj := text[start : end+1]
		if pm := pathPropRE.FindStringSubmatch(obj); len(pm) == 2 {
			return normalizeNestedPath(parentRoutePath(text, start), pm[1])
		}
		search = start - 1
	}
	return ""
}

func matching(text string, open int, left, right byte) int {
	depth := 0
	for i := open; i < len(text); i++ {
		if text[i] == left {
			depth++
		} else if text[i] == right {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func routeComponent(objectText string) string {
	if m := componentPropRE.FindStringSubmatch(objectText); len(m) == 2 {
		component := strings.TrimSpace(m[1])
		component = strings.Trim(component, "{}< />")
		return component
	}
	return ""
}

func normalizeNestedPath(parent, child string) string {
	if parent == "" {
		return normalizeRoutePath(child)
	}
	if child == "" || child == "/" {
		return normalizeRoutePath(parent)
	}
	if strings.HasPrefix(child, "/") {
		return normalizeRoutePath(child)
	}
	return astutil.JoinPath(parent, child)
}

func normalizeRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func (e *ReactRouterExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	return symbol != nil && (strings.Contains(strings.ToLower(symbol.Signature), "jsx") || strings.HasSuffix(symbol.Name, "Page"))
}
