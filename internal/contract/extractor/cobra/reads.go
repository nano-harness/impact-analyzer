package cobra

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// mapAddrToFreeVar finds which FreeVar in a closure corresponds to a variable address
// Returns the FreeVar and its index in the Bindings array, or nil if not found
func mapAddrToFreeVar(closure *ssa.MakeClosure, varAddr ssa.Value) (*ssa.FreeVar, int) {
	if closure == nil || varAddr == nil {
		return nil, -1
	}

	// Get the function from the closure
	fn, ok := closure.Fn.(*ssa.Function)
	if !ok || fn == nil {
		return nil, -1
	}

	// Linear search through bindings
	for i, binding := range closure.Bindings {
		if binding == varAddr {
			// Found the binding at index i
			if i < len(fn.FreeVars) {
				return fn.FreeVars[i], i
			}
		}
	}

	return nil, -1
}

// findFreeVarLoads finds all load operations (*ssa.UnOp with Op=token.MUL) on a FreeVar
func findFreeVarLoads(freeVar *ssa.FreeVar) []ssa.Value {
	var loads []ssa.Value

	if freeVar == nil {
		return loads
	}

	// Iterate through all uses of this FreeVar
	if referrers := freeVar.Referrers(); referrers != nil {
		for _, instr := range *referrers {
			// Check for dereference (load) operation
			if unop, ok := instr.(*ssa.UnOp); ok && unop.Op == token.MUL {
				loads = append(loads, unop)
			}
		}
	}

	return loads
}

// findReadsInNamedFunc finds read points for a variable address in a named RunE function
// This is the Layer 3 dispatcher that routes to specialized handlers
func findReadsInNamedFunc(varAddr ssa.Value, runEFn *ssa.Function, calleeDepth int) []ssa.Value {
	if varAddr == nil || runEFn == nil {
		return nil
	}

	switch v := varAddr.(type) {
	case *ssa.Global:
		return findGlobalReadsInFunc(v, runEFn, calleeDepth)
	case *ssa.FieldAddr:
		return findFieldReadsInMethod(v, runEFn, calleeDepth)
	case *ssa.Alloc:
		// Local alloc only valid if defined in the same function
		if v.Parent() == runEFn {
			return findAllocReadsInFunc(v)
		}
		return nil
	default:
		return nil
	}
}

// findGlobalReadsInFunc finds all reads of a global variable in a function and its callees
func findGlobalReadsInFunc(global *ssa.Global, fn *ssa.Function, calleeDepth int) []ssa.Value {
	var reads []ssa.Value
	visited := make(map[*ssa.Function]bool)

	collectGlobalReads(global, fn, calleeDepth, visited, &reads)

	return reads
}

// collectGlobalReads recursively collects global variable reads
func collectGlobalReads(global *ssa.Global, fn *ssa.Function, depth int, visited map[*ssa.Function]bool, reads *[]ssa.Value) {
	if fn == nil || visited[fn] {
		return
	}
	visited[fn] = true

	if fn.Blocks == nil {
		return
	}

	// Find reads in current function
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			// Check for dereference of the global
			if unop, ok := instr.(*ssa.UnOp); ok && unop.Op == token.MUL {
				if unop.X == global {
					*reads = append(*reads, unop)
				}
			}

			// Recurse into callees if depth allows
			if depth > 0 {
				if call, ok := instr.(*ssa.Call); ok && !call.Call.IsInvoke() {
					if callee, ok := call.Call.Value.(*ssa.Function); ok {
						// Skip if no package (external/builtin)
						if callee.Pkg == nil || callee.Pkg.Pkg == nil {
							continue
						}
						// Skip stdlib
						if isStdlibFunc(callee) {
							continue
						}
						// Skip IO/tool functions
						if isIOToolFunc(callee) {
							continue
						}
						collectGlobalReads(global, callee, depth-1, visited, reads)
					}
				}
			}
		}
	}
}

// findFieldReadsInMethod finds all reads of a struct field in a method and its callees
// This implements Layer 2 fallback algorithm
func findFieldReadsInMethod(fa *ssa.FieldAddr, runEFn *ssa.Function, calleeDepth int) []ssa.Value {
	if fa == nil || runEFn == nil {
		return nil
	}

	srcType := unwrapPointer(fa.X.Type())
	fieldIdx := fa.Field

	var reads []ssa.Value
	visited := make(map[*ssa.Function]bool)

	collectFieldReads(srcType, fieldIdx, runEFn, calleeDepth, visited, &reads)

	return reads
}

// collectFieldReads recursively collects field reads matching the source type and field index
func collectFieldReads(srcType types.Type, fieldIdx int, fn *ssa.Function, depth int, visited map[*ssa.Function]bool, reads *[]ssa.Value) {
	if fn == nil || visited[fn] {
		return
	}
	visited[fn] = true

	if fn.Blocks == nil {
		return
	}

	// Find field reads in current function
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			// Check for FieldAddr matching our field
			if fieldAddr, ok := instr.(*ssa.FieldAddr); ok {
				if fieldAddr.Field == fieldIdx {
					// Verify owner type matches
					ownerType := unwrapPointer(fieldAddr.X.Type())
					if types.Identical(ownerType, srcType) {
						// Found matching field access, collect its loads
						if referrers := fieldAddr.Referrers(); referrers != nil {
							for _, ref := range *referrers {
								if unop, ok := ref.(*ssa.UnOp); ok && unop.Op == token.MUL {
									*reads = append(*reads, unop)
								}
							}
						}
					}
				}
			}

			// Recurse into callees if depth allows
			if depth > 0 {
				if call, ok := instr.(*ssa.Call); ok && !call.Call.IsInvoke() {
					if callee, ok := call.Call.Value.(*ssa.Function); ok {
						// Skip if no package
						if callee.Pkg == nil || callee.Pkg.Pkg == nil {
							continue
						}
						// Skip stdlib
						if isStdlibFunc(callee) {
							continue
						}
						// Skip IO/tool functions
						if isIOToolFunc(callee) {
							continue
						}
						collectFieldReads(srcType, fieldIdx, callee, depth-1, visited, reads)
					}
				}
			}
		}
	}
}

// findAllocReadsInFunc finds all reads of a local allocation within its defining function
func findAllocReadsInFunc(alloc *ssa.Alloc) []ssa.Value {
	var reads []ssa.Value

	if alloc == nil {
		return reads
	}

	// Find all dereference operations on this alloc
	if referrers := alloc.Referrers(); referrers != nil {
		for _, instr := range *referrers {
			if unop, ok := instr.(*ssa.UnOp); ok && unop.Op == token.MUL {
				reads = append(reads, unop)
			}
		}
	}

	return reads
}

// unwrapPointer removes pointer indirection from a type
func unwrapPointer(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// isStdlibFunc checks if a function is from the standard library
// Stdlib packages don't contain "." in their import path
func isStdlibFunc(fn *ssa.Function) bool {
	if fn == nil || fn.Pkg == nil || fn.Pkg.Pkg == nil {
		return false
	}
	// Stdlib import paths don't have dots (e.g., "fmt", "strings", "os/exec")
	// vs. github.com/... or golang.org/x/...
	pkgPath := fn.Pkg.Pkg.Path()
	return pkgPath != "" && !contains(pkgPath, ".")
}

// isIOToolFunc checks if a function is from an IO/tool library
// Uses the whitelist from §1.8
func isIOToolFunc(fn *ssa.Function) bool {
	if fn == nil || fn.Pkg == nil || fn.Pkg.Pkg == nil {
		return false
	}

	pkgPath := fn.Pkg.Pkg.Path()

	// Check prefixes
	for _, prefix := range ioToolPathPrefixes {
		if hasPrefix(pkgPath, prefix) {
			return true
		}
	}

	// Check substrings
	for _, substr := range ioToolPathSubstrings {
		if contains(pkgPath, substr) {
			return true
		}
	}

	return false
}

// IO/tool whitelist (from §1.8)
var ioToolPathPrefixes = []string{
	"github.com/spf13/cobra",
	"github.com/spf13/pflag",
	"github.com/dustin/go-humanize",
	"go.uber.org/zap",
	"github.com/sirupsen/logrus",
	"golang.org/x/",
	"google.golang.org/",
}

var ioToolPathSubstrings = []string{
	"/iostreams",
	"/cliutil",
	"/iostream",
	"/output",
	"/format",
	"/print",
}

// Helper string functions
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
