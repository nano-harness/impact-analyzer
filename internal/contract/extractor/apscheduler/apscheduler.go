package apscheduler

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type APSchedulerExtractor struct{ *simple.RegexExtractor }

func New() *APSchedulerExtractor {
	return &APSchedulerExtractor{&simple.RegexExtractor{ExtractorName: "apscheduler", FrameworkName: "apscheduler", Lang: model.LanguagePython, ContractKind: model.ContractKindScheduledJob, Extensions: []string{".py"}, Markers: []string{"APScheduler", "add_job", "scheduled_job"}, Patterns: []simple.Pattern{simple.MustPattern(`add_job\(([^,)]+)`, model.ContractKindScheduledJob, "JOB", 0, 1), simple.MustPattern(`@\w+\.scheduled_job\(`, model.ContractKindScheduledJob, "JOB", 0, 0)}}}
}
func (e *APSchedulerExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *APSchedulerExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *APSchedulerExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *APSchedulerExtractor) Language() model.Language {
	e.ensure()
	return e.RegexExtractor.Language()
}
func (e *APSchedulerExtractor) CanHandle(f string) bool {
	e.ensure()
	return e.RegexExtractor.CanHandle(f)
}
func (e *APSchedulerExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *APSchedulerExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *APSchedulerExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *APSchedulerExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
