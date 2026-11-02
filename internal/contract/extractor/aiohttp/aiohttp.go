package aiohttp

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type AioHTTPExtractor struct{ *simple.RegexExtractor }

func New() *AioHTTPExtractor {
	return &AioHTTPExtractor{&simple.RegexExtractor{ExtractorName: "aiohttp", FrameworkName: "aiohttp", Lang: model.LanguagePython, ContractKind: model.ContractKindHTTPRoute, Extensions: []string{".py"}, Markers: []string{"aiohttp", "starlette", "sanic", "add_routes", "add_route"}, Patterns: []simple.Pattern{simple.MustPattern(`add_(?:route|routes)\(.*["']([^"']+)["'].*?,\s*([^,)]+)`, model.ContractKindHTTPRoute, "ROUTE", 1, 2), simple.MustPattern(`@\w+\.route\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "ROUTE", 1, 0)}}}
}
func (e *AioHTTPExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *AioHTTPExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *AioHTTPExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *AioHTTPExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *AioHTTPExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *AioHTTPExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *AioHTTPExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *AioHTTPExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *AioHTTPExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
