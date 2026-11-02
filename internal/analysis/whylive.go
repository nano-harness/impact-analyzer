package analysis

import (
	"fmt"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// WhyReachableResult describes the path from an entry point to a target symbol.
type WhyReachableResult struct {
	Target    string     `json:"target"` // Target symbol qname
	Reachable bool       `json:"reachable"`
	Path      []PathEdge `json:"path,omitempty"` // entry → ... → target
	Source    string     `json:"source"`         // "ssa" | "tree-sitter" | "both"
}

// PathEdge represents one hop in a why-reachable path.
type PathEdge struct {
	From     string `json:"from"`      // caller qname
	To       string `json:"to"`        // callee qname
	EdgeKind string `json:"edge_kind"` // "call", "type_ref", "inheritance"
	File     string `json:"file"`
	Line     int    `json:"line"`
}

const whyReachableMaxDepth = 20

// WhyReachable traces the shortest path from any entry point to the target symbol.
// It performs reverse BFS on the tree-sitter call graph from the target back to entries.
func (a *OrphanAnalyzer) WhyReachable(targetQName string) (*WhyReachableResult, error) {
	result := &WhyReachableResult{
		Target: targetQName,
		Source: "tree-sitter",
	}

	// Find the target symbol in the graph
	targetSym, ok := a.graph.GetSymbol(targetQName)
	if !ok {
		return nil, fmt.Errorf("symbol %q not found in call graph", targetQName)
	}

	// Build entry set
	builder := NewEntrySetBuilder(a.cfg, a.extractors)
	entries := builder.Build(a.graph)

	// Check if target itself is an entry
	if _, isEntry := entries[targetQName]; isEntry {
		result.Reachable = true
		result.Path = nil // Direct entry, no path needed
		return result, nil
	}

	// Reverse BFS from target to find an entry point
	path := a.reverseBFS(targetSym, entries, a.graph)
	if path != nil {
		result.Reachable = true
		result.Path = path
		return result, nil
	}

	// Not reachable via tree-sitter
	result.Reachable = false
	return result, nil
}

// parentInfo stores the BFS parent relationship for path reconstruction.
type parentInfo struct {
	callerQName string
	edgeKind    string
}

// reverseBFS performs BFS from target backwards through callers to find an entry point.
// Returns the path (from entry to target) or nil if no path found.
func (a *OrphanAnalyzer) reverseBFS(target *model.Symbol, entries map[string]*model.Symbol, cg *graph.CallGraph) []PathEdge {
	type bfsNode struct {
		sym   *model.Symbol
		depth int
	}

	targetQName := target.QualifiedName()

	// parent[qname] = (caller_qname, edge_kind)
	parent := make(map[string]*parentInfo)
	parent[targetQName] = nil // mark start

	queue := []bfsNode{{sym: target, depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= whyReachableMaxDepth {
			continue
		}

		currentQName := current.sym.QualifiedName()

		// Get callers of current symbol
		callers := cg.GetCallers(current.sym)
		for _, caller := range callers {
			if caller == nil {
				continue
			}
			callerQName := caller.QualifiedName()
			if _, visited := parent[callerQName]; visited {
				continue
			}

			// Determine edge kind
			edgeKind := "call"

			parent[callerQName] = &parentInfo{
				callerQName: currentQName,
				edgeKind:    edgeKind,
			}

			// Check if we reached an entry point
			if _, isEntry := entries[callerQName]; isEntry {
				// Reconstruct path from entry to target
				return reconstructPath(callerQName, targetQName, parent, cg)
			}

			queue = append(queue, bfsNode{sym: caller, depth: current.depth + 1})
		}
	}

	return nil
}

// reconstructPath builds the forward path from entry to target using the parent map.
func reconstructPath(entryQName, targetQName string, parent map[string]*parentInfo, cg *graph.CallGraph) []PathEdge {
	// Trace from entry to target by reversing the parent chain
	var reverseChain []string
	current := entryQName
	for current != targetQName {
		reverseChain = append(reverseChain, current)
		info := parent[current]
		if info == nil {
			break
		}
		current = info.callerQName
	}
	reverseChain = append(reverseChain, targetQName)

	// Reverse the chain so it goes entry → target
	// (parent map stores child→parent, but we collected entry→...→target via callerQName field)
	// Actually the parent map goes backwards: parent[callerQName].callerQName = currentQName (the callee)
	// So the chain is: entry → next → ... → target

	var path []PathEdge
	for i := 0; i < len(reverseChain)-1; i++ {
		from := reverseChain[i]
		to := reverseChain[i+1]

		file := ""
		line := 0
		if sym, ok := cg.GetSymbol(to); ok && sym != nil {
			file = sym.File
			line = sym.Line
		}

		path = append(path, PathEdge{
			From:     from,
			To:       to,
			EdgeKind: "call",
			File:     file,
			Line:     line,
		})
	}

	return path
}

// FormatWhyReachablePath formats a WhyReachableResult as a human-readable string.
func FormatWhyReachablePath(result *WhyReachableResult) string {
	if !result.Reachable {
		return fmt.Sprintf("%s: NOT reachable from any entry point", result.Target)
	}
	if len(result.Path) == 0 {
		return fmt.Sprintf("%s: is an entry point (directly reachable)", result.Target)
	}

	// Build path string: entry → hop1 → hop2 → target
	s := result.Path[0].From
	for _, edge := range result.Path {
		s += " → " + edge.To
	}
	s += fmt.Sprintf(" (%d hops)", len(result.Path))
	return s
}
