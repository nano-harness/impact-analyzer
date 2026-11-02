package typer

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type TyperExtractor struct{ *simple.RegexExtractor }

func New() *TyperExtractor {
	return &TyperExtractor{&simple.RegexExtractor{ExtractorName: "typer", FrameworkName: "typer", Lang: model.LanguagePython, ContractKind: model.ContractKindCLICommand, Extensions: []string{".py"}, Markers: []string{"typer.Typer", "@app.command", ".command("}, Patterns: []simple.Pattern{simple.MustPattern(`@\w+\.command`, model.ContractKindCLICommand, "", 0, 1)}}}
}
func (e *TyperExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *TyperExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *TyperExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *TyperExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *TyperExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *TyperExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *TyperExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *TyperExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *TyperExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
