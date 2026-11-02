package nodecron

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type NodeCronExtractor struct{ *simple.RegexExtractor }

func New() *NodeCronExtractor {
	return &NodeCronExtractor{&simple.RegexExtractor{ExtractorName: "nodecron", FrameworkName: "node-cron", Lang: model.LanguageJavaScript, ContractKind: model.ContractKindScheduledJob, Extensions: []string{".js", ".jsx", ".ts", ".tsx"}, Markers: []string{"node-cron", "node-schedule", "@Cron", "@Interval", "@Timeout", "cron.schedule"}, Patterns: []simple.Pattern{simple.MustPattern(`(?:cron|schedule)\.schedule\([^,]+,\s*([A-Za-z_][A-Za-z0-9_\.]*)`, model.ContractKindScheduledJob, "CRON", 0, 1), simple.MustPattern(`@(Cron|Interval|Timeout)\(`, model.ContractKindScheduledJob, "JOB", 0, 1)}}}
}
func (e *NodeCronExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *NodeCronExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *NodeCronExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *NodeCronExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *NodeCronExtractor) CanHandle(f string) bool {
	e.ensure()
	return e.RegexExtractor.CanHandle(f)
}
func (e *NodeCronExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *NodeCronExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *NodeCronExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *NodeCronExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
