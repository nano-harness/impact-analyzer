package click

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type ClickExtractor struct{ *simple.RegexExtractor }

func New() *ClickExtractor {
	return &ClickExtractor{&simple.RegexExtractor{ExtractorName: "click", FrameworkName: "click", Lang: model.LanguagePython, ContractKind: model.ContractKindCLICommand, Extensions: []string{".py"}, Markers: []string{"click.", "@click."}, Patterns: []simple.Pattern{simple.MustPattern(`@click\.(command|group)`, model.ContractKindCLICommand, "", 0, 1)}}}
}
func (e *ClickExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *ClickExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *ClickExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *ClickExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *ClickExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *ClickExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *ClickExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *ClickExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *ClickExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
