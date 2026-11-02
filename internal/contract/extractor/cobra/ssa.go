package cobra

import (
	"fmt"
	"go/types"
	"os"
	"os/exec"

	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/ssa"
)

// SSAExtractor extracts cobra commands using SSA analysis for advanced patterns
type SSAExtractor struct {
	enabled bool
}

// NewSSAExtractor creates a new SSA-based cobra extractor
func NewSSAExtractor() *SSAExtractor {
	return &SSAExtractor{
		enabled: false, // Disabled by default, can be enabled by caller
	}
}

// SetEnabled enables or disables SSA extraction
func (e *SSAExtractor) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// IsEnabled returns true if SSA extraction is enabled
func (e *SSAExtractor) IsEnabled() bool {
	return e.enabled
}

// ExtractFromPackage extracts cobra contracts from a package using SSA
func (e *SSAExtractor) ExtractFromPackage(pkgPath string) ([]*model.Contract, error) {
	if !e.enabled {
		return nil, nil
	}

	prog, _, err := LoadPackageSSA(pkgPath)
	if err != nil {
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/ssa] LoadPackageSSA failed: %v\n", err)
		}
		return nil, err
	}

	if prog == nil {
		return nil, fmt.Errorf("SSA program is nil")
	}

	// Derive expected import path from directory
	expectedPkgPath := currentImportPath(pkgPath)
	if expectedPkgPath == "" {
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/ssa] currentImportPath failed, continuing without filtering\n")
		}
	}

	var contracts []*model.Contract

	// Pattern A: Factory functions
	contracts = append(contracts, e.extractPatternA(prog, expectedPkgPath)...)

	// Pattern W: Global commands (Part 2)
	contracts = append(contracts, e.extractPatternW(prog, expectedPkgPath)...)

	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[cobra/ssa] Extracted %d total contracts\n", len(contracts))
	}

	return contracts, nil
}

// extractPatternA extracts contracts from factory functions (Pattern A)
func (e *SSAExtractor) extractPatternA(prog *ssa.Program, expectedPkgPath string) []*model.Contract {
	var contracts []*model.Contract

	factories := findCobraFactories(prog)
	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[cobra/ssa] Pattern A: Found %d factory functions\n", len(factories))
	}

	for _, fn := range factories {
		// Skip factories not in the target package
		if expectedPkgPath != "" && fn.Pkg != nil && fn.Pkg.Pkg != nil {
			if fn.Pkg.Pkg.Path() != expectedPkgPath {
				if debugEnabled() {
					fmt.Fprintf(os.Stderr, "[cobra/ssa] Skipping %s (wrong package: %s != %s)\n",
						funcQName(fn), fn.Pkg.Pkg.Path(), expectedPkgPath)
				}
				continue
			}
		}

		// Classify RunE
		target := classifyRunETarget(fn)
		if target.Kind == RunENone || target.Kind == RunEUnknown {
			if debugEnabled() {
				fmt.Fprintf(os.Stderr, "[cobra/ssa] Skipping %s (RunE kind: %d)\n", funcQName(fn), target.Kind)
			}
			continue
		}

		// Extract flag bindings
		bindings := findFlagBindings(fn)
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/ssa] %s: found %d flag bindings\n", funcQName(fn), len(bindings))
		}

		// Extract command metadata
		use, short, long, aliases, line, file := extractCommandMetadata(fn)
		if use == "" {
			if debugEnabled() {
				fmt.Fprintf(os.Stderr, "[cobra/ssa] Skipping %s (no Use field)\n", funcQName(fn))
			}
			continue
		}

		// Build contract
		contract := e.buildContractFromBindings(use, short, long, aliases, line, file, bindings, target)
		if contract != nil {
			contracts = append(contracts, contract)
		}
	}

	return contracts
}

// extractPatternW extracts contracts from global commands (Pattern W)
func (e *SSAExtractor) extractPatternW(prog *ssa.Program, expectedPkgPath string) []*model.Contract {
	var contracts []*model.Contract

	globals := findGlobalCobraCommands(prog)
	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[cobra/ssa] Pattern W: Found %d global commands\n", len(globals))
	}

	for _, g := range globals {
		// Skip globals not in the target package
		if expectedPkgPath != "" && g.Pkg != nil && g.Pkg.Pkg != nil {
			if g.Pkg.Pkg.Path() != expectedPkgPath {
				if debugEnabled() {
					fmt.Fprintf(os.Stderr, "[cobra/ssa] Skipping %s (wrong package)\n", g.Name())
				}
				continue
			}
		}

		// Extract metadata
		use, short, long, aliases, line, file, runEFn := extractGlobalCommandMetadata(g, prog)
		if use == "" {
			if debugEnabled() {
				fmt.Fprintf(os.Stderr, "[cobra/ssa] Skipping %s (no Use field)\n", g.Name())
			}
			continue
		}

		// Find flag bindings across the program
		bindings := findFlagBindingsForGlobal(prog, g)
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/ssa] %s: found %d flag bindings\n", g.Name(), len(bindings))
		}

		// Build contract with named RunE
		target := RunETarget{
			Kind:    RunENamed,
			NamedFn: runEFn,
		}
		contract := e.buildContractFromBindings(use, short, long, aliases, line, file, bindings, target)
		if contract != nil {
			contracts = append(contracts, contract)
		}
	}

	return contracts
}

// buildContractFromBindings builds a contract from bindings and RunE target
func (e *SSAExtractor) buildContractFromBindings(use, short, long string, aliases []string, line int, file string, bindings []FlagBinding, target RunETarget) *model.Contract {
	// Build contract signature
	signature := make(map[string]interface{})
	signature["use"] = use
	if short != "" {
		signature["short"] = short
	}
	if long != "" {
		signature["long"] = long
	}
	if len(aliases) > 0 {
		signature["aliases"] = aliases
	}

	// Process flag bindings
	var flags []map[string]interface{}
	var allHandlers []map[string]interface{}

	for _, b := range bindings {
		// Skip non-literal flag names (Part 1 constraint)
		if b.FlagName == "" {
			if debugEnabled() {
				fmt.Fprintf(os.Stderr, "[cobra/ssa] Skipping binding with empty flag name\n")
			}
			continue
		}

		// Find read points
		var readPoints []ssa.Value
		switch target.Kind {
		case RunEAnonymous, RunEFactoryClosure:
			// Layer 1: Try FreeVar direct match
			freeVar, _ := mapAddrToFreeVar(target.Closure, b.VarAddr)
			if freeVar != nil {
				readPoints = findFreeVarLoads(freeVar)
				if debugEnabled() {
					fmt.Fprintf(os.Stderr, "[cobra/ssa] Layer 1: found %d read points for %s\n",
						len(readPoints), b.FlagName)
				}
			} else if fa, ok := b.VarAddr.(*ssa.FieldAddr); ok && target.AnonFn != nil {
				// Layer 2: FieldAddr fallback
				readPoints = findFieldReadsInMethod(fa, target.AnonFn, 5)
				if debugEnabled() {
					fmt.Fprintf(os.Stderr, "[cobra/ssa] Layer 2: found %d read points for %s\n",
						len(readPoints), b.FlagName)
				}
			}
		case RunENamed:
			if target.NamedFn != nil {
				// Layer 3: Named function dispatch
				readPoints = findReadsInNamedFunc(b.VarAddr, target.NamedFn, 5)
				if debugEnabled() {
					fmt.Fprintf(os.Stderr, "[cobra/ssa] Layer 3: found %d read points for %s\n",
						len(readPoints), b.FlagName)
				}
			}
		}

		// Part 2: Collect handlers from dataflow
		for _, rp := range readPoints {
			usages := traceDataflow(rp, 5)
			for _, u := range usages {
				if u.Kind == "call_arg" && u.Callee != nil {
					handler := map[string]interface{}{
						"flag":      b.FlagName,
						"callee":    funcQName(u.Callee),
						"arg_index": u.ArgIndex,
						"depth":     u.Depth,
					}
					allHandlers = append(allHandlers, handler)
				}
			}
		}

		// Add flag to signature
		flagData := map[string]interface{}{
			"name": b.FlagName,
			"type": b.FlagType,
		}
		flags = append(flags, flagData)
	}

	if len(flags) > 0 {
		signature["flags"] = flags
	}

	// Normalize and add handlers (Part 2)
	if len(allHandlers) > 0 {
		signature["handlers"] = normalizeHandlers(allHandlers)
	}

	// Build contract
	contract := &model.Contract{
		Kind:       model.ContractKindCLICommand,
		Identifier: use,
		File:       file,
		Line:       line,
		Signature:  signature,
		Framework:  "cobra",
		Language:   model.LanguageGo,
	}

	// Compute signature hash
	if err := contract.ComputeSignatureHash(); err != nil {
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/ssa] Failed to compute hash for %s: %v\n", use, err)
		}
		return nil
	}

	return contract
}

// extractCommandMetadata extracts Use, Short, Long, Aliases from a factory function
func extractCommandMetadata(fn *ssa.Function) (use, short, long string, aliases []string, line int, file string) {
	if fn == nil || fn.Blocks == nil {
		return
	}

	// Find cobra.Command allocation
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			alloc, ok := instr.(*ssa.Alloc)
			if !ok {
				continue
			}

			// Check if allocating cobra.Command
			if !isCobraCommandAlloc(alloc) {
				continue
			}

			// Record position
			if alloc.Pos().IsValid() && fn.Prog != nil {
				pos := fn.Prog.Fset.Position(alloc.Pos())
				line = pos.Line
				file = pos.Filename
			}

			// Find field stores to this allocation using the corrected SSA
			// referrer chain for &cobra.Command{Use: "..."}:
			//   alloc -> FieldAddr(Use) -> Store(addr=FieldAddr, val="...")
			if alloc.Referrers() == nil {
				continue
			}

			extractCommandMetadataFromAlloc(alloc, &use, &short, &long, &aliases, nil)

			// Found the allocation, return
			return
		}
	}

	return
}

// extractCommandMetadataFromAlloc extracts cobra.Command field values from SSA stores to alloc field addresses.
func extractCommandMetadataFromAlloc(alloc *ssa.Alloc, use, short, long *string, aliases *[]string, runEFn **ssa.Function) {
	if alloc == nil || alloc.Referrers() == nil {
		return
	}

	for _, ref := range *alloc.Referrers() {
		fieldAddr, ok := ref.(*ssa.FieldAddr)
		if !ok || fieldAddr.X != alloc || fieldAddr.Referrers() == nil {
			continue
		}

		fieldName := getFieldName(fieldAddr)
		if fieldName == "" {
			continue
		}

		for _, faRef := range *fieldAddr.Referrers() {
			store, ok := faRef.(*ssa.Store)
			if !ok || store.Addr != fieldAddr {
				continue
			}

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
			case "Aliases":
				if len(*aliases) == 0 {
					*aliases = extractStringSliceFromValue(store.Val)
				}
			case "RunE", "Run":
				if runEFn != nil && *runEFn == nil {
					if namedFn, ok := store.Val.(*ssa.Function); ok {
						*runEFn = namedFn
					}
				}
			}
		}
	}
}

// isCobraCommandAlloc checks if an allocation is for cobra.Command
func isCobraCommandAlloc(alloc *ssa.Alloc) bool {
	if alloc == nil {
		return false
	}

	t := alloc.Type()
	if t == nil {
		return false
	}

	// Check if it's *cobra.Command or **cobra.Command
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}

	// Handle **cobra.Command (double pointer)
	if innerPtr, ok := ptr.Elem().(*types.Pointer); ok {
		return isCobraCommandPointer(innerPtr)
	}

	// Handle *cobra.Command
	return isCobraCommandPointer(ptr)
}

// getFieldName gets the field name from a FieldAddr instruction
func getFieldName(fieldAddr *ssa.FieldAddr) string {
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

// extractConstantValue extracts constant value from SSA value
func extractConstantValue(val ssa.Value) interface{} {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case *ssa.Const:
		if v.Value != nil {
			// Remove quotes from string constants
			s := v.Value.ExactString()
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
				return s[1 : len(s)-1]
			}
			return s
		}
	case *ssa.MakeInterface:
		return extractConstantValue(v.X)
	case *ssa.ChangeType:
		return extractConstantValue(v.X)
	case *ssa.Convert:
		return extractConstantValue(v.X)
	}

	return nil
}

// extractStringSliceFromSlice attempts to extract string literals from a slice construction
func extractStringSliceFromSlice(slice *ssa.Slice) []string {
	if slice == nil || slice.X == nil {
		return nil
	}

	if alloc, ok := slice.X.(*ssa.Alloc); ok {
		return extractStringSliceFromArrayAlloc(alloc)
	}

	return nil
}

// currentImportPath derives the Go import path from a directory
// Returns empty string on failure (caller should continue without filtering)
func currentImportPath(dir string) string {
	// Reuse the pkgPathFromDir logic but without error exposure
	pkgPath, _ := pkgPathFromDir(dir)
	return pkgPath
}

// pkgPathFromDir derives the Go import path from a directory
func pkgPathFromDir(dir string) (string, error) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}")
	cmd.Dir = dir

	// Suppress stderr
	cmd.Stderr = nil

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	pkgPath := string(output)
	// Trim whitespace
	for len(pkgPath) > 0 && (pkgPath[len(pkgPath)-1] == '\n' || pkgPath[len(pkgPath)-1] == '\r' || pkgPath[len(pkgPath)-1] == ' ' || pkgPath[len(pkgPath)-1] == '\t') {
		pkgPath = pkgPath[:len(pkgPath)-1]
	}

	return pkgPath, nil
}

// debugEnabled checks if debug mode is enabled
func debugEnabled() bool {
	return os.Getenv("IMPACT_DEBUG_COBRA") == "1"
}

// normalizeHandlers deduplicates and sorts handlers by (flag, callee, arg_index)
func normalizeHandlers(handlers []map[string]interface{}) []map[string]interface{} {
	if len(handlers) == 0 {
		return handlers
	}

	// Deduplicate by (flag, callee, arg_index), keeping min depth
	type key struct {
		flag     string
		callee   string
		argIndex int
	}
	seen := make(map[key]map[string]interface{})

	for _, h := range handlers {
		flag, _ := h["flag"].(string)
		callee, _ := h["callee"].(string)
		argIndex, _ := h["arg_index"].(int)
		depth, _ := h["depth"].(int)

		k := key{flag: flag, callee: callee, argIndex: argIndex}
		if existing, ok := seen[k]; ok {
			// Keep handler with smaller depth
			existingDepth, _ := existing["depth"].(int)
			if depth < existingDepth {
				seen[k] = h
			}
		} else {
			seen[k] = h
		}
	}

	// Convert back to slice
	var result []map[string]interface{}
	for _, h := range seen {
		result = append(result, h)
	}

	// Sort by (flag, callee, arg_index)
	sortHandlers(result)

	return result
}

// sortHandlers sorts handlers by (flag, callee, arg_index) lexicographically
func sortHandlers(handlers []map[string]interface{}) {
	// Simple bubble sort for small slices
	n := len(handlers)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if compareHandlers(handlers[j], handlers[j+1]) > 0 {
				handlers[j], handlers[j+1] = handlers[j+1], handlers[j]
			}
		}
	}
}

// compareHandlers compares two handlers for sorting
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareHandlers(a, b map[string]interface{}) int {
	flagA, _ := a["flag"].(string)
	flagB, _ := b["flag"].(string)
	if flagA != flagB {
		if flagA < flagB {
			return -1
		}
		return 1
	}

	calleeA, _ := a["callee"].(string)
	calleeB, _ := b["callee"].(string)
	if calleeA != calleeB {
		if calleeA < calleeB {
			return -1
		}
		return 1
	}

	argIndexA, _ := a["arg_index"].(int)
	argIndexB, _ := b["arg_index"].(int)
	if argIndexA < argIndexB {
		return -1
	} else if argIndexA > argIndexB {
		return 1
	}

	return 0
}
