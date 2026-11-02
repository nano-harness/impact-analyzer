package analysis

import (
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ReachState represents the tri-state reachability of a symbol.
type ReachState int

const (
	Unreachable ReachState = iota
	WeakReach              // Only reachable via TypeRef/Decorator edges
	StrongReach            // Reachable via at least one Call/Inheritance edge
)

// ReachabilityMarker marks symbols reachable from entry points via caller -> callee edges.
type ReachabilityMarker struct{ graph *graph.CallGraph }

func NewReachabilityMarker(cg *graph.CallGraph) *ReachabilityMarker {
	return &ReachabilityMarker{graph: cg}
}

func (m *ReachabilityMarker) Mark(entries map[string]*model.Symbol) map[string]bool {
	reachable := make(map[string]bool)
	queue := make([]*model.Symbol, 0, len(entries))
	for qname, entry := range entries {
		if entry == nil {
			continue
		}
		reachable[qname] = true
		queue = append(queue, entry)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, callee := range m.graph.GetCallees(current) {
			if callee == nil {
				continue
			}
			qname := callee.QualifiedName()
			if reachable[qname] {
				continue
			}
			reachable[qname] = true
			queue = append(queue, callee)
		}
	}
	return reachable
}

// MarkTriState performs BFS with edge-kind awareness returning tri-state reachability.
// First pass: traverse Call + Inheritance edges for StrongReach.
// Second pass: traverse TypeRef/Decorator edges on unvisited nodes for WeakReach.
func (m *ReachabilityMarker) MarkTriState(entries map[string]*model.Symbol) map[string]ReachState {
	states := make(map[string]ReachState)

	// First pass: BFS via Call + Inheritance edges only → StrongReach
	queue := make([]*model.Symbol, 0, len(entries))
	for qname, entry := range entries {
		if entry == nil {
			continue
		}
		states[qname] = StrongReach
		queue = append(queue, entry)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, callee := range m.graph.GetCalleesByKind(current, model.EdgeKindCall) {
			if callee == nil {
				continue
			}
			qname := callee.QualifiedName()
			if _, visited := states[qname]; visited {
				continue
			}
			states[qname] = StrongReach
			queue = append(queue, callee)
		}
		for _, callee := range m.graph.GetCalleesByKind(current, model.EdgeKindInheritance) {
			if callee == nil {
				continue
			}
			qname := callee.QualifiedName()
			if _, visited := states[qname]; visited {
				continue
			}
			states[qname] = StrongReach
			queue = append(queue, callee)
		}
	}

	// Second pass: BFS via TypeRef + Decorator edges on not-yet-visited nodes → WeakReach
	weakQueue := make([]*model.Symbol, 0)
	// Seed from all StrongReach nodes
	for qname := range states {
		if sym, ok := m.graph.GetSymbol(qname); ok {
			weakQueue = append(weakQueue, sym)
		}
	}
	for len(weakQueue) > 0 {
		current := weakQueue[0]
		weakQueue = weakQueue[1:]
		for _, callee := range m.graph.GetCalleesByKind(current, model.EdgeKindTypeRef) {
			if callee == nil {
				continue
			}
			qname := callee.QualifiedName()
			if _, visited := states[qname]; visited {
				continue
			}
			states[qname] = WeakReach
			weakQueue = append(weakQueue, callee)
		}
		for _, callee := range m.graph.GetCalleesByKind(current, model.EdgeKindDecorator) {
			if callee == nil {
				continue
			}
			qname := callee.QualifiedName()
			if _, visited := states[qname]; visited {
				continue
			}
			states[qname] = WeakReach
			weakQueue = append(weakQueue, callee)
		}
	}

	return states
}
