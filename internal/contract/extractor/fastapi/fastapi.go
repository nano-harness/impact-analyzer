package fastapi

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type FastAPIExtractor struct{ *simple.RegexExtractor }

func New() *FastAPIExtractor {
	return &FastAPIExtractor{&simple.RegexExtractor{ExtractorName: "fastapi", FrameworkName: "fastapi", Lang: model.LanguagePython, ContractKind: model.ContractKindHTTPRoute, Extensions: []string{".py"}, Markers: []string{"FastAPI", "APIRouter", "@app.", "@router."}, Patterns: []simple.Pattern{simple.MustPattern(`@(?:app|router)\.get\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "GET", 1, 0), simple.MustPattern(`@(?:app|router)\.post\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "POST", 1, 0), simple.MustPattern(`@(?:app|router)\.put\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "PUT", 1, 0), simple.MustPattern(`@(?:app|router)\.delete\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "DELETE", 1, 0), simple.MustPattern(`@(?:app|router)\.patch\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "PATCH", 1, 0), simple.MustPattern(`include_router\(([^,)]+)`, model.ContractKindHTTPRoute, "ROUTER", 0, 1)}}}
}
func (e *FastAPIExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *FastAPIExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *FastAPIExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *FastAPIExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *FastAPIExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *FastAPIExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *FastAPIExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *FastAPIExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *FastAPIExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
