package tornado

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type TornadoExtractor struct{ *simple.RegexExtractor }

func New() *TornadoExtractor {
	return &TornadoExtractor{&simple.RegexExtractor{ExtractorName: "tornado", FrameworkName: "tornado", Lang: model.LanguagePython, ContractKind: model.ContractKindHTTPRoute, Extensions: []string{".py"}, Markers: []string{"tornado", "RequestHandler"}, Patterns: []simple.Pattern{simple.MustPattern(`class\s+([A-Za-z_][A-Za-z0-9_]*)\(.*RequestHandler`, model.ContractKindHTTPRoute, "HANDLER", 0, 1)}}}
}
func (e *TornadoExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *TornadoExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *TornadoExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *TornadoExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *TornadoExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *TornadoExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *TornadoExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *TornadoExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *TornadoExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
