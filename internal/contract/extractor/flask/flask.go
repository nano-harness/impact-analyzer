package flask

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type FlaskExtractor struct{ *simple.RegexExtractor }

func New() *FlaskExtractor {
	return &FlaskExtractor{&simple.RegexExtractor{ExtractorName: "flask", FrameworkName: "flask", Lang: model.LanguagePython, ContractKind: model.ContractKindHTTPRoute, Extensions: []string{".py"}, Markers: []string{"Flask", "Blueprint", ".route("}, Patterns: []simple.Pattern{simple.MustPattern(`@\w+\.route\(["']([^"']+)["']`, model.ContractKindHTTPRoute, "ROUTE", 1, 0), simple.MustPattern(`class\s+([A-Za-z_][A-Za-z0-9_]*)\(.*MethodView`, model.ContractKindHTTPRoute, "VIEW", 0, 1)}}}
}
func (e *FlaskExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *FlaskExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *FlaskExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *FlaskExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *FlaskExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *FlaskExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *FlaskExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *FlaskExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *FlaskExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
