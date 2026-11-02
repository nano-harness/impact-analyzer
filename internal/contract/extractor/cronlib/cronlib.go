package cronlib

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type CronlibExtractor struct{ *simple.RegexExtractor }

func New() *CronlibExtractor {
	return &CronlibExtractor{&simple.RegexExtractor{ExtractorName: "cronlib", FrameworkName: "cron", Lang: model.LanguageGo, ContractKind: model.ContractKindScheduledJob, Extensions: []string{".go"}, Markers: []string{"robfig/cron", "AddFunc", "time.AfterFunc", "go func"}, Patterns: []simple.Pattern{simple.MustPattern(`AddFunc\([^,]+,\s*([A-Za-z_][A-Za-z0-9_\.]*)`, model.ContractKindScheduledJob, "CRON", 0, 1), simple.MustPattern(`time\.AfterFunc\([^,]+,\s*([A-Za-z_][A-Za-z0-9_\.]*)`, model.ContractKindScheduledJob, "TIMER", 0, 1)}}}
}
func (e *CronlibExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *CronlibExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *CronlibExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *CronlibExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *CronlibExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *CronlibExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *CronlibExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *CronlibExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *CronlibExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
