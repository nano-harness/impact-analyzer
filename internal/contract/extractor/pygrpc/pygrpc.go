package pygrpc

import (
	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/simple"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type PyGRPCExtractor struct{ *simple.RegexExtractor }

func New() *PyGRPCExtractor {
	return &PyGRPCExtractor{&simple.RegexExtractor{ExtractorName: "pygrpc", FrameworkName: "grpcio", Lang: model.LanguagePython, ContractKind: model.ContractKindGRPCService, Extensions: []string{".py"}, Markers: []string{"grpc", "Servicer"}, Patterns: []simple.Pattern{simple.MustPattern(`class\s+([A-Za-z_][A-Za-z0-9_]*)\(.*Servicer`, model.ContractKindGRPCService, "", 0, 1)}}}
}
func (e *PyGRPCExtractor) ensure() {
	if e.RegexExtractor == nil {
		*e = *New()
	}
}
func (e *PyGRPCExtractor) Name() string             { e.ensure(); return e.RegexExtractor.Name() }
func (e *PyGRPCExtractor) Kind() model.ContractKind { e.ensure(); return e.RegexExtractor.Kind() }
func (e *PyGRPCExtractor) Language() model.Language { e.ensure(); return e.RegexExtractor.Language() }
func (e *PyGRPCExtractor) CanHandle(f string) bool  { e.ensure(); return e.RegexExtractor.CanHandle(f) }
func (e *PyGRPCExtractor) Extract(f string) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.Extract(f)
}
func (e *PyGRPCExtractor) CanHandleContent(f string, content []byte) bool {
	e.ensure()
	return e.RegexExtractor.CanHandleContent(f, content)
}
func (e *PyGRPCExtractor) ExtractContent(f string, content []byte) ([]*model.Contract, error) {
	e.ensure()
	return e.RegexExtractor.ExtractContent(f, content)
}
func (e *PyGRPCExtractor) IsEntryPoint(s *model.Symbol) bool {
	e.ensure()
	return e.RegexExtractor.IsEntryPoint(s)
}
