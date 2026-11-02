package cobra

import (
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// RunEKind represents the kind of RunE field assignment
type RunEKind int

const (
	RunENone           RunEKind = iota // No RunE field found
	RunEAnonymous                      // RunE: func(){...} (anonymous function)
	RunENamed                          // RunE: runCR (named function)
	RunEFactoryClosure                 // RunE: makeRunE(args) (factory returns closure)
	RunEUnknown                        // Cannot classify
)

// RunETarget holds the classified RunE assignment information
type RunETarget struct {
	Kind     RunEKind
	AnonFn   *ssa.Function    // Set when Kind == RunEAnonymous or RunEFactoryClosure
	NamedFn  *ssa.Function    // Set when Kind == RunENamed
	Closure  *ssa.MakeClosure // Set when Kind == RunEAnonymous or RunEFactoryClosure
	StoreVal ssa.Value        // The raw value stored to RunE field
}

// findCobraFactories finds functions that return *cobra.Command
func findCobraFactories(prog *ssa.Program) []*ssa.Function {
	var factories []*ssa.Function

	if prog == nil {
		return factories
	}

	for _, pkg := range prog.AllPackages() {
		if pkg == nil {
			continue
		}

		for _, mem := range pkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok || fn == nil {
				continue
			}

			if returnsCobraCommand(fn) {
				factories = append(factories, fn)
			}
		}
	}

	return factories
}

// returnsCobraCommand checks if a function returns *cobra.Command
func returnsCobraCommand(fn *ssa.Function) bool {
	if fn == nil || fn.Signature == nil {
		return false
	}

	results := fn.Signature.Results()
	if results == nil || results.Len() == 0 {
		return false
	}

	// Check first return value is *cobra.Command
	return isCobraCommandPointer(results.At(0).Type())
}

// isCobraCommandPointer checks if a type is *cobra.Command
func isCobraCommandPointer(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}

	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}

	return pkg.Path() == "github.com/spf13/cobra" && obj.Name() == "Command"
}

// classifyRunETarget analyzes a factory function and classifies its RunE assignment
func classifyRunETarget(fn *ssa.Function) RunETarget {
	target := RunETarget{Kind: RunENone}

	if fn == nil || fn.Blocks == nil {
		return target
	}

	// Find cobra.Command allocation and its RunE field store
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok {
				continue
			}

			if !isRunEFieldStore(store) {
				continue
			}

			target.StoreVal = store.Val

			// Case 1: MakeClosure (anonymous function)
			if mc, ok := store.Val.(*ssa.MakeClosure); ok {
				target.Kind = RunEAnonymous
				target.Closure = mc
				if fn, ok := mc.Fn.(*ssa.Function); ok {
					target.AnonFn = fn
				}
				return target
			}

			// Case 2: Named function
			if namedFn, ok := store.Val.(*ssa.Function); ok {
				target.Kind = RunENamed
				target.NamedFn = namedFn
				return target
			}

			// Case 3: Factory closure (call returns closure)
			if call, ok := store.Val.(*ssa.Call); ok {
				mc, anonFn := resolveFactoryReturnedClosure(call)
				if mc != nil && anonFn != nil {
					target.Kind = RunEFactoryClosure
					target.Closure = mc
					target.AnonFn = anonFn
					return target
				}
			}

			// Found RunE store but cannot classify
			target.Kind = RunEUnknown
			return target
		}
	}

	return target
}

// isRunEFieldStore checks if a Store instruction writes to the RunE field of cobra.Command
func isRunEFieldStore(store *ssa.Store) bool {
	if store == nil {
		return false
	}

	fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
	if !ok {
		return false
	}

	// Get the struct type
	t := fieldAddr.X.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	// Check if it's cobra.Command
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil || pkg.Path() != "github.com/spf13/cobra" || obj.Name() != "Command" {
		return false
	}

	// Get field name
	structType, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	if fieldAddr.Field >= 0 && fieldAddr.Field < structType.NumFields() {
		fieldName := structType.Field(fieldAddr.Field).Name()
		return fieldName == "RunE"
	}

	return false
}

// resolveFactoryReturnedClosure traces a call to see if it returns a closure
// Returns the MakeClosure and its anonymous function if found
func resolveFactoryReturnedClosure(call *ssa.Call) (*ssa.MakeClosure, *ssa.Function) {
	if call == nil || call.Call.Value == nil {
		return nil, nil
	}

	callee, ok := call.Call.Value.(*ssa.Function)
	if !ok || callee == nil || callee.Blocks == nil {
		return nil, nil
	}

	// Look for return statement in callee
	for _, block := range callee.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok || len(ret.Results) == 0 {
				continue
			}

			// Check if first return value is MakeClosure
			if mc, ok := ret.Results[0].(*ssa.MakeClosure); ok {
				if fn, ok := mc.Fn.(*ssa.Function); ok {
					return mc, fn
				}
			}
		}
	}

	return nil, nil
}

// isAnonymous checks if a function is anonymous (not package-level)
func isAnonymous(fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	// Anonymous functions have "$" in their name
	return strings.Contains(fn.Name(), "$")
}

// funcQName returns a qualified name for a function
func funcQName(fn *ssa.Function) string {
	if fn == nil {
		return "<nil>"
	}
	if fn.Pkg != nil && fn.Pkg.Pkg != nil {
		return fn.Pkg.Pkg.Path() + "." + fn.Name()
	}
	return fn.Name()
}

// findGlobalCobraCommands finds package-level cobra.Command variables (Pattern W)
func findGlobalCobraCommands(prog *ssa.Program) []*ssa.Global {
	var globals []*ssa.Global

	if prog == nil {
		return globals
	}

	for _, pkg := range prog.AllPackages() {
		if pkg == nil {
			continue
		}

		for _, mem := range pkg.Members {
			g, ok := mem.(*ssa.Global)
			if !ok || g == nil {
				continue
			}

			// Check if global's type is *cobra.Command
			// Global.Type() returns **T for a var of type *T
			if ptr, ok := g.Type().(*types.Pointer); ok {
				if isCobraCommandPointer(ptr) {
					globals = append(globals, g)
				}
			}
		}
	}

	return globals
}

// extractGlobalCommandMetadata extracts metadata from a global cobra.Command variable
func extractGlobalCommandMetadata(g *ssa.Global, prog *ssa.Program) (use, short, long string, aliases []string, line int, file string, runEFn *ssa.Function) {
	if g == nil || prog == nil || g.Pkg == nil {
		return
	}

	// Find all stores to this global's fields
	// Need to scan all functions in the package (especially init functions)
	pkg := g.Pkg
	if pkg == nil {
		return
	}

	// Scan all functions in the package
	for _, mem := range pkg.Members {
		fn, ok := mem.(*ssa.Function)
		if !ok || fn == nil || fn.Blocks == nil {
			continue
		}

		extractMetadataFromFunction(g, fn, &use, &short, &long, &aliases, &line, &file, &runEFn, prog)
	}

	return
}

// extractMetadataFromFunction extracts command metadata from stores in a function
func extractMetadataFromFunction(g *ssa.Global, fn *ssa.Function, use, short, long *string, aliases *[]string, line *int, file *string, runEFn **ssa.Function, prog *ssa.Program) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if alloc, ok := instr.(*ssa.Alloc); ok && isCobraCommandAlloc(alloc) && isAllocStoredToGlobal(alloc, g) {
				if *line == 0 && alloc.Pos().IsValid() && prog.Fset != nil {
					pos := prog.Fset.Position(alloc.Pos())
					*line = pos.Line
					*file = pos.Filename
				}
				extractCommandMetadataFromAlloc(alloc, use, short, long, aliases, runEFn)
				continue
			}

			store, ok := instr.(*ssa.Store)
			if !ok {
				continue
			}

			// Check if this is a store to a field of our global command
			fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
			if !ok {
				continue
			}

			// Check if FieldAddr's X is a load of our global
			// X could be UnOp{MUL, Global} or directly Global
			var isOurGlobal bool
			if unop, ok := fieldAddr.X.(*ssa.UnOp); ok {
				if unop.X == g {
					isOurGlobal = true
				}
			} else if fieldAddr.X == g {
				isOurGlobal = true
			}

			if !isOurGlobal {
				continue
			}

			// Record position if not set yet
			if *line == 0 && store.Pos().IsValid() && prog.Fset != nil {
				pos := prog.Fset.Position(store.Pos())
				*line = pos.Line
				*file = pos.Filename
			}

			// Extract field value based on field name
			fieldName := getFieldNameFromFieldAddr(fieldAddr)
			val := extractConstantValue(store.Val)

			switch fieldName {
			case "Use":
				if s, ok := val.(string); ok && *use == "" {
					*use = s
				}
			case "Short":
				if s, ok := val.(string); ok && *short == "" {
					*short = s
				}
			case "Long":
				if s, ok := val.(string); ok && *long == "" {
					*long = s
				}
			case "RunE", "Run":
				// Extract RunE/Run function
				if namedFn, ok := store.Val.(*ssa.Function); ok && *runEFn == nil {
					*runEFn = namedFn
				}
			case "Aliases":
				// Try to extract aliases (simplified for Part 2)
				if len(*aliases) == 0 {
					*aliases = extractStringSliceFromValue(store.Val)
				}
			}
		}
	}
}

func isAllocStoredToGlobal(alloc *ssa.Alloc, g *ssa.Global) bool {
	if alloc == nil || g == nil || alloc.Referrers() == nil {
		return false
	}

	for _, ref := range *alloc.Referrers() {
		store, ok := ref.(*ssa.Store)
		if ok && store.Addr == g && store.Val == alloc {
			return true
		}
	}

	return false
}

// getFieldNameFromFieldAddr gets the field name from a FieldAddr instruction
func getFieldNameFromFieldAddr(fieldAddr *ssa.FieldAddr) string {
	if fieldAddr == nil {
		return ""
	}

	t := fieldAddr.X.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	structType, ok := t.Underlying().(*types.Struct)
	if !ok {
		return ""
	}

	if fieldAddr.Field >= 0 && fieldAddr.Field < structType.NumFields() {
		return structType.Field(fieldAddr.Field).Name()
	}

	return ""
}

// extractStringSliceFromValue attempts to extract []string from an SSA value
func extractStringSliceFromValue(val ssa.Value) []string {
	for {
		switch v := val.(type) {
		case *ssa.Slice:
			return extractStringSliceFromSlice(v)
		case *ssa.ChangeType:
			val = v.X
		case *ssa.Convert:
			val = v.X
		case *ssa.MakeInterface:
			val = v.X
		default:
			return nil
		}
	}
}

func extractStringSliceFromArrayAlloc(alloc *ssa.Alloc) []string {
	if alloc == nil || alloc.Referrers() == nil {
		return nil
	}

	values := make(map[int]string)
	maxIndex := -1
	for _, ref := range *alloc.Referrers() {
		indexAddr, ok := ref.(*ssa.IndexAddr)
		if !ok || indexAddr.X != alloc || indexAddr.Referrers() == nil {
			continue
		}
		indexConst, ok := indexAddr.Index.(*ssa.Const)
		if !ok || indexConst.Value == nil {
			continue
		}
		idx64, exact := constant.Int64Val(indexConst.Value)
		if !exact || idx64 < 0 {
			continue
		}
		idx := int(idx64)
		for _, idxRef := range *indexAddr.Referrers() {
			store, ok := idxRef.(*ssa.Store)
			if !ok || store.Addr != indexAddr {
				continue
			}
			if s, ok := extractConstantValue(store.Val).(string); ok {
				values[idx] = s
				if idx > maxIndex {
					maxIndex = idx
				}
			}
		}
	}

	if maxIndex < 0 {
		return nil
	}

	// Preserve SSA array indexes; any missing store remains the string zero value.
	result := make([]string, maxIndex+1)
	for i := 0; i <= maxIndex; i++ {
		if s, ok := values[i]; ok {
			result[i] = s
		}
	}

	return result
}
