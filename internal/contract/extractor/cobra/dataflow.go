package cobra

import (
	"golang.org/x/tools/go/ssa"
)

// Usage represents how a value is used in the dataflow
type Usage struct {
	Kind     string        // "call_arg", "interface_invoke", "field_store", "branch_cond", "expr_use", "stop_stdlib", "stop_io"
	Callee   *ssa.Function // For "call_arg": the called function
	ArgIndex int           // For "call_arg": argument position
	Depth    int           // Trace depth
	Note     string        // Human-readable description
}

// traceDataflow performs BFS dataflow tracing from a start value
func traceDataflow(startVal ssa.Value, maxDepth int) []Usage {
	var usages []Usage

	if startVal == nil {
		return usages
	}

	queue := []workItem{{val: startVal, depth: 0}}
	visited := make(map[ssa.Value]bool)

	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]

		if visited[w.val] || w.depth > maxDepth {
			continue
		}
		visited[w.val] = true

		if w.val.Referrers() == nil {
			continue
		}

		for _, instr := range *w.val.Referrers() {
			switch inst := instr.(type) {
			case *ssa.Call:
				handleCallUsage(inst, w, &usages, &queue)

			case *ssa.Extract:
				// Tuple extraction, continue tracing
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.Phi:
				// Phi node, continue tracing
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.MakeInterface:
				// Interface wrapping, continue tracing
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.TypeAssert:
				// Type assertion, continue tracing
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.ChangeType:
				// Type change, continue tracing
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.Convert:
				// Conversion, continue tracing
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.Store:
				// Store operation
				target := classifyStoreTarget(inst)
				usages = append(usages, Usage{
					Kind:  target,
					Depth: w.depth,
					Note:  storeTargetDesc(inst),
				})

			case *ssa.If:
				// Branch condition
				usages = append(usages, Usage{
					Kind:  "branch_cond",
					Depth: w.depth,
					Note:  "used in branch condition",
				})

			case *ssa.BinOp:
				// Binary operation, continue tracing
				usages = append(usages, Usage{
					Kind:  "expr_use",
					Depth: w.depth,
					Note:  "binary operation",
				})
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.UnOp:
				// Unary operation, continue tracing
				usages = append(usages, Usage{
					Kind:  "expr_use",
					Depth: w.depth,
					Note:  "unary operation",
				})
				queue = append(queue, workItem{val: inst, depth: w.depth})

			case *ssa.Return:
				// Return statement
				usages = append(usages, Usage{
					Kind:  "expr_use",
					Depth: w.depth,
					Note:  "returned from function",
				})

			case *ssa.Go:
				// Goroutine spawn (not traced further)
				usages = append(usages, Usage{
					Kind:  "expr_use",
					Depth: w.depth,
					Note:  "passed to goroutine",
				})

			case *ssa.Defer:
				// Defer statement (not traced further)
				usages = append(usages, Usage{
					Kind:  "expr_use",
					Depth: w.depth,
					Note:  "passed to defer",
				})

			default:
				// Other uses
				usages = append(usages, Usage{
					Kind:  "expr_use",
					Depth: w.depth,
					Note:  "other usage",
				})
			}
		}
	}

	return usages
}

// workItem represents an item in the BFS queue
type workItem struct {
	val   ssa.Value
	depth int
}

// handleCallUsage processes a call instruction in the dataflow trace
func handleCallUsage(call *ssa.Call, w workItem, usages *[]Usage, queue *[]workItem) {
	// Handle interface invoke
	if call.Call.IsInvoke() {
		*usages = append(*usages, Usage{
			Kind:  "interface_invoke",
			Depth: w.depth,
			Note:  "passed to interface method",
		})
		return
	}

	callee, ok := call.Call.Value.(*ssa.Function)
	if !ok || callee == nil {
		return
	}

	// Find argument position
	argIdx := -1
	for i, arg := range call.Call.Args {
		if arg == w.val {
			argIdx = i
			break
		}
	}

	if argIdx < 0 {
		return
	}

	// Check if stdlib
	if isStdlibFunc(callee) {
		*usages = append(*usages, Usage{
			Kind:     "stop_stdlib",
			Callee:   callee,
			ArgIndex: argIdx,
			Depth:    w.depth,
			Note:     "passed to stdlib: " + funcQName(callee),
		})
		return
	}

	// Check if IO/tool
	if isIOToolFunc(callee) {
		*usages = append(*usages, Usage{
			Kind:     "stop_io",
			Callee:   callee,
			ArgIndex: argIdx,
			Depth:    w.depth,
			Note:     "passed to IO/tool: " + funcQName(callee),
		})
		return
	}

	// Regular call - trace into parameter
	*usages = append(*usages, Usage{
		Kind:     "call_arg",
		Callee:   callee,
		ArgIndex: argIdx,
		Depth:    w.depth + 1,
		Note:     "arg " + intToString(argIdx) + " of " + funcQName(callee),
	})

	// Continue tracing through callee's parameter
	if argIdx < len(callee.Params) {
		*queue = append(*queue, workItem{
			val:   callee.Params[argIdx],
			depth: w.depth + 1,
		})
	}
}

// classifyStoreTarget classifies what kind of store target this is
func classifyStoreTarget(s *ssa.Store) string {
	if s == nil {
		return "expr_use"
	}

	switch s.Addr.(type) {
	case *ssa.FieldAddr:
		return "field_store"
	case *ssa.IndexAddr:
		return "array_store"
	case *ssa.Global:
		return "global_store"
	case *ssa.Alloc:
		return "local_store"
	default:
		return "expr_use"
	}
}

// storeTargetDesc returns a human-readable description of store target
func storeTargetDesc(s *ssa.Store) string {
	if s == nil {
		return "unknown store"
	}

	switch addr := s.Addr.(type) {
	case *ssa.FieldAddr:
		return "stored to field"
	case *ssa.IndexAddr:
		return "stored to array element"
	case *ssa.Global:
		if addr.Name() != "" {
			return "stored to global " + addr.Name()
		}
		return "stored to global"
	case *ssa.Alloc:
		if addr.Name() != "" {
			return "stored to local " + addr.Name()
		}
		return "stored to local"
	default:
		return "stored to memory"
	}
}

// intToString converts int to string without importing strconv
func intToString(n int) string {
	if n < 0 {
		return "-" + intToString(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return intToString(n/10) + string(rune('0'+n%10))
}
