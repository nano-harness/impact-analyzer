package graph

// DepGraph represents package/module dependencies
type DepGraph struct {
	// edges maps package -> []dependencies
	edges map[string][]string
}

// NewDepGraph creates a new dependency graph
func NewDepGraph() *DepGraph {
	return &DepGraph{
		edges: make(map[string][]string),
	}
}

// AddDependency adds a dependency edge
func (dg *DepGraph) AddDependency(from, to string) {
	dg.edges[from] = append(dg.edges[from], to)
}

// GetDependencies returns direct dependencies of a package
func (dg *DepGraph) GetDependencies(pkg string) []string {
	return dg.edges[pkg]
}

// GetReverseDependencies returns packages that depend on the given package
func (dg *DepGraph) GetReverseDependencies(pkg string) []string {
	var result []string
	for p, deps := range dg.edges {
		for _, dep := range deps {
			if dep == pkg {
				result = append(result, p)
				break
			}
		}
	}
	return result
}
