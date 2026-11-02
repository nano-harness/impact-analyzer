package simple

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

type Pattern struct {
	Regex  *regexp.Regexp
	Kind   model.ContractKind
	Method string
	// PathIdx and NameIdx use regexp submatch indexes. Use 0 when the pattern
	// does not provide that semantic capture; use 1+ to select capture groups.
	PathIdx int
	NameIdx int
}

type RegexExtractor struct {
	ExtractorName string
	FrameworkName string
	Lang          model.Language
	ContractKind  model.ContractKind
	Extensions    []string
	Markers       []string
	Patterns      []Pattern
}

func (e *RegexExtractor) Name() string             { return e.ExtractorName }
func (e *RegexExtractor) Kind() model.ContractKind { return e.ContractKind }
func (e *RegexExtractor) Language() model.Language { return e.Lang }

func (e *RegexExtractor) CanHandle(filePath string) bool {
	if !e.matchExt(filePath) {
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
func (e *RegexExtractor) CanHandleContent(filePath string, content []byte) bool {
	if !e.matchExt(filePath) {
		return false
	}
	text := string(content)
	for _, marker := range e.Markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return len(e.Markers) == 0
}

func (e *RegexExtractor) matchExt(filePath string) bool {
	return len(e.Extensions) == 0 || contains(e.Extensions, filepath.Ext(filePath))
}

func (e *RegexExtractor) Extract(filePath string) ([]*model.Contract, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return e.ExtractContent(filePath, content)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *RegexExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	lines := strings.Split(string(content), "\n")
	var contracts []*model.Contract
	seen := map[string]bool{}
	for i, line := range lines {
		for _, p := range e.Patterns {
			m := p.Regex.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			kind := p.Kind
			if kind == "" {
				kind = e.ContractKind
			}
			name := group(m, p.NameIdx)
			path := group(m, p.PathIdx)
			method := p.Method
			identifier := name
			if kind == model.ContractKindHTTPRoute {
				if method == "" {
					method = "ROUTE"
				}
				if path == "" {
					path = "/" + strings.TrimPrefix(name, "/")
				}
				identifier = strings.TrimSpace(method + " " + path)
			}
			if identifier == "" {
				identifier = fmt.Sprintf("%s:%d", e.FrameworkName, i+1)
			}
			key := string(kind) + ":" + identifier
			if seen[key] {
				continue
			}
			seen[key] = true
			sig := map[string]interface{}{"framework": e.FrameworkName}
			if name != "" {
				sig["handler"] = cleanName(name)
				sig["function"] = cleanName(name)
				sig["task"] = cleanName(name)
			}
			if path != "" {
				sig["path"] = path
			}
			if method != "" {
				sig["method"] = method
			}
			c := &model.Contract{Kind: kind, Identifier: identifier, File: filePath, Line: i + 1, Signature: sig, Framework: e.FrameworkName, Language: e.Lang}
			if err := c.ComputeSignatureHash(); err == nil {
				contracts = append(contracts, c)
			}
		}
	}
	return contracts, nil
}

func (e *RegexExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	return symbol != nil && symbol.Language == e.Lang && (strings.Contains(strings.ToLower(filepath.Base(symbol.File)), e.ExtractorName) || strings.Contains(symbol.Signature, e.FrameworkName))
}

func MustPattern(expr string, kind model.ContractKind, method string, pathIdx, nameIdx int) Pattern {
	return Pattern{Regex: regexp.MustCompile(expr), Kind: kind, Method: method, PathIdx: pathIdx, NameIdx: nameIdx}
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func group(m []string, idx int) string {
	// Non-positive indexes mean "no semantic capture". Index 0 is the regexp
	// full match and is intentionally not returned by this helper.
	if idx <= 0 || idx >= len(m) {
		return ""
	}
	return strings.Trim(m[idx], `"' `)
}
func cleanName(name string) string {
	parts := strings.Split(strings.TrimSpace(name), ".")
	return strings.Trim(parts[len(parts)-1], `"' `)
}
