package celery

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type CeleryExtractor struct{ *simple.RegexExtractor }

func New() *CeleryExtractor {
	return &CeleryExtractor{&simple.RegexExtractor{ExtractorName: "celery", FrameworkName: "celery", Lang: model.LanguagePython, ContractKind: model.ContractKindScheduledJob, Extensions: []string{".py"}, Markers: []string{"Celery", "shared_task", ".task", "beat_schedule"}, Patterns: []simple.Pattern{simple.MustPattern(`@(?:\w+\.)?task\b|@shared_task\b`, model.ContractKindScheduledJob, "TASK", 0, 0), simple.MustPattern(`beat_schedule`, model.ContractKindScheduledJob, "SCHEDULE", 0, 0)}}}
}
func (e *CeleryExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *CeleryExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *CeleryExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *CeleryExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *CeleryExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *CeleryExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *CeleryExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *CeleryExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *CeleryExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
