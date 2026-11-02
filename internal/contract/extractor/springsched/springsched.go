package springsched

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type SpringSchedExtractor struct{ *simple.RegexExtractor }

func New() *SpringSchedExtractor {
	return &SpringSchedExtractor{&simple.RegexExtractor{ExtractorName: "springsched", FrameworkName: "spring-scheduled", Lang: model.LanguageJava, ContractKind: model.ContractKindScheduledJob, Extensions: []string{".java"}, Markers: []string{"@Scheduled", "@KafkaListener", "@RabbitListener", "@EventListener", "@XxlJob", "QuartzJobBean"}, Patterns: []simple.Pattern{simple.MustPattern(`@(Scheduled|KafkaListener|RabbitListener|EventListener|XxlJob)`, model.ContractKindScheduledJob, "JOB", 0, 1), simple.MustPattern(`class\s+([A-Za-z_][A-Za-z0-9_]*)\s+.*Job`, model.ContractKindScheduledJob, "JOB", 0, 1)}}}
}
func (e *SpringSchedExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *SpringSchedExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *SpringSchedExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *SpringSchedExtractor) Language() model.Language {
	e.ensure()
	return e.RegexExtractor.Language()
}
func (e *SpringSchedExtractor) CanHandle(f string) bool {
	e.ensure()
	return e.RegexExtractor.CanHandle(f)
}
func (e *SpringSchedExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *SpringSchedExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *SpringSchedExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *SpringSchedExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
