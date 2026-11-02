package django

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type DjangoExtractor struct{ *simple.RegexExtractor }

func New() *DjangoExtractor {
	return &DjangoExtractor{&simple.RegexExtractor{ExtractorName: "django", FrameworkName: "django", Lang: model.LanguagePython, ContractKind: model.ContractKindHTTPRoute, Extensions: []string{".py"}, Markers: []string{"django", "urlpatterns", "path(", "re_path(", "api_view", "ViewSet"}, Patterns: []simple.Pattern{simple.MustPattern(`(?:path|re_path)\(["']([^"']+)["']\s*,\s*([^,)]+)`, model.ContractKindHTTPRoute, "ROUTE", 1, 2), simple.MustPattern(`@api_view\(`, model.ContractKindHTTPRoute, "API", 0, 0), simple.MustPattern(`class\s+([A-Za-z_][A-Za-z0-9_]*)\(.*(?:View|ViewSet)`, model.ContractKindHTTPRoute, "VIEW", 0, 1)}}}
}
func (e *DjangoExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *DjangoExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *DjangoExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *DjangoExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *DjangoExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *DjangoExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *DjangoExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *DjangoExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *DjangoExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
