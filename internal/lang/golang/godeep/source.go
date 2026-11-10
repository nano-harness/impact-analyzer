package godeep

import "golang.org/x/tools/go/ssa"

// SourceFunction returns the declaration behind an instantiated function.
// Instances may have a nil Pkg and names such as identity[int]; reports and
// reachability keys must instead use the source package and declaration name.
// The call graph itself continues to contain the instantiated functions.
func SourceFunction(fn *ssa.Function) *ssa.Function {
	if fn != nil {
		if origin := fn.Origin(); origin != nil {
			return origin
		}
	}
	return fn
}
