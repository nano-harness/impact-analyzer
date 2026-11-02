package diff

import (
	"sort"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

const (
	FallbackStructField = "struct_field"
	FallbackVarRef      = "var_ref"
	FallbackFileLevel   = "file_level"
)

// OutOfFuncResolver resolves non-function changes to safe BFS start points.
type OutOfFuncResolver struct {
	callGraph *graph.CallGraph
}

func NewOutOfFuncResolver(cg *graph.CallGraph) *OutOfFuncResolver {
	return &OutOfFuncResolver{callGraph: cg}
}

func (r *OutOfFuncResolver) Resolve(fileSymbols *model.FileSymbols, lineNum int) ([]*model.Symbol, string) {
	sym := findEnclosingSymbol(fileSymbols, lineNum)
	if sym == nil {
		return nil, FallbackFileLevel
	}

	switch sym.Kind {
	case model.SymbolKindStruct, model.SymbolKindInterface, model.SymbolKindClass:
		return []*model.Symbol{sym}, FallbackStructField
	case model.SymbolKindVariable, model.SymbolKindConstant:
		if r.callGraph != nil {
			callers := r.callGraph.GetCallers(sym)
			if len(callers) > 0 {
				return callers, FallbackVarRef
			}
		}
		return []*model.Symbol{sym}, FallbackVarRef
	default:
		return nil, FallbackFileLevel
	}
}

func findEnclosingSymbol(fileSymbols *model.FileSymbols, lineNum int) *model.Symbol {
	if fileSymbols == nil {
		return nil
	}
	var best *model.Symbol
	for _, sym := range fileSymbols.Symbols {
		if !symbolContainsLine(sym, lineNum) {
			continue
		}
		if best == nil || symbolSpan(sym) < symbolSpan(best) || (symbolSpan(sym) == symbolSpan(best) && sym.Line > best.Line) {
			best = sym
		}
	}
	return best
}

func sortedSymbols(fileSymbols *model.FileSymbols) []*model.Symbol {
	if fileSymbols == nil {
		return nil
	}
	symbols := append([]*model.Symbol(nil), fileSymbols.Symbols...)
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].File != symbols[j].File {
			return symbols[i].File < symbols[j].File
		}
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].QualifiedName() < symbols[j].QualifiedName()
	})
	return symbols
}

func symbolContainsLine(sym *model.Symbol, lineNum int) bool {
	if sym == nil || lineNum < sym.Line {
		return false
	}
	if sym.EndLine == 0 {
		return lineNum == sym.Line
	}
	return lineNum <= sym.EndLine
}

func symbolSpan(sym *model.Symbol) int {
	if sym == nil || sym.EndLine <= sym.Line {
		return 1
	}
	return sym.EndLine - sym.Line + 1
}
