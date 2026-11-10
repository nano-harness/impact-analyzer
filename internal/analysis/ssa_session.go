package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/lang/golang/godeep"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/callgraph"
)

// SSASession holds a single SSA/RTA analysis result that can be shared between
// the orphan reachability check and the call graph edge injection paths.
// It ensures SSA is loaded at most once per analysis run.
type SSASession struct {
	repoRoot          string
	customEntryPoints []string
	buildTags         []string
	reflectionAware   bool

	analyzer  *godeep.Analyzer
	callGraph *callgraph.Graph
	reachable map[PosKey]bool
	rootCount int
	err       error
	once      sync.Once
}

// NewSSASession creates a new SSA session for the given repository root.
func NewSSASession(repoRoot string, customEntryPoints, buildTags []string, reflectionAware bool) *SSASession {
	return &SSASession{
		repoRoot:          repoRoot,
		customEntryPoints: customEntryPoints,
		buildTags:         buildTags,
		reflectionAware:   reflectionAware,
	}
}

// EnsureBuilt loads packages, builds SSA, and computes the call graph exactly once.
func (s *SSASession) EnsureBuilt() error {
	s.once.Do(func() {
		s.err = s.build()
	})
	return s.err
}

func (s *SSASession) build() error {
	// Verify repo has go.mod
	goModPath := filepath.Join(s.repoRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("no go.mod found at %s", goModPath)
	}

	// Create analyzer with build tags
	s.analyzer = godeep.NewAnalyzer()
	if len(s.buildTags) > 0 {
		s.analyzer = s.analyzer.WithBuildTags(s.buildTags)
	}

	// Convert repoRoot to absolute path
	absRoot, err := filepath.Abs(s.repoRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Load all packages in the repository
	pattern := absRoot + "/..."
	if err := s.analyzer.LoadPackages(pattern); err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	// Build SSA representation
	if err := s.analyzer.BuildSSA(); err != nil {
		return fmt.Errorf("failed to build SSA: %w", err)
	}

	// Build RTA call graph with optional custom entry points
	cg, err := s.analyzer.BuildCallGraph(s.customEntryPoints...)
	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}

	// Apply reflection-conservative model if enabled
	if s.reflectionAware {
		s.analyzer.MarkReflectionReachable(cg)
	}

	s.callGraph = cg

	// Count roots (entry points in the call graph)
	s.rootCount = countRoots(cg)

	// Extract reachable function positions
	positions := s.analyzer.ReachableFunctionPositions(cg)
	s.reachable = make(map[PosKey]bool, len(positions))
	for pos := range positions {
		s.reachable[PosKey{File: pos.File, Line: pos.Line, Name: pos.Name}] = true
	}

	return nil
}

// Reachable returns the set of reachable function positions.
func (s *SSASession) Reachable() (map[PosKey]bool, error) {
	if err := s.EnsureBuilt(); err != nil {
		return nil, err
	}
	return s.reachable, nil
}

// CallGraph returns the computed call graph.
func (s *SSASession) CallGraph() (*callgraph.Graph, error) {
	if err := s.EnsureBuilt(); err != nil {
		return nil, err
	}
	return s.callGraph, nil
}

// RootCount returns the number of entry point roots used in the analysis.
func (s *SSASession) RootCount() int {
	return s.rootCount
}

// ReachableCount returns the number of reachable functions.
func (s *SSASession) ReachableCount() int {
	return len(s.reachable)
}

// CallEdgesForDir returns call edges for functions defined in the given directory.
func (s *SSASession) CallEdgesForDir(dir string) ([]model.CallEdge, error) {
	if err := s.EnsureBuilt(); err != nil {
		return nil, err
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	var edges []model.CallEdge
	for fn, node := range s.callGraph.Nodes {
		if fn == nil || node == nil {
			continue
		}
		fn = godeep.SourceFunction(fn)
		pos := s.analyzer.Prog.Fset.Position(fn.Pos())
		if pos.Filename == "" {
			continue
		}
		fnDir := filepath.Dir(pos.Filename)
		if fnDir != absDir {
			continue
		}
		for _, edge := range node.Out {
			if edge.Callee == nil || edge.Callee.Func == nil {
				continue
			}
			callee := godeep.SourceFunction(edge.Callee.Func)
			calleePos := s.analyzer.Prog.Fset.Position(callee.Pos())
			edges = append(edges, model.CallEdge{
				CallerFile: pos.Filename,
				CallerLine: pos.Line,
				CallerName: fn.Name(),
				CalleeFile: calleePos.Filename,
				CalleeLine: calleePos.Line,
				CalleeName: callee.Name(),
			})
		}
	}
	return edges, nil
}

// Release frees the SSA program memory.
func (s *SSASession) Release() {
	s.analyzer = nil
	s.callGraph = nil
}

// countRoots counts the number of root nodes (nodes with no incoming edges) in the call graph.
func countRoots(cg *callgraph.Graph) int {
	if cg == nil {
		return 0
	}
	// If the graph has a designated root, count nodes reachable only from root
	// Otherwise count nodes that have no incoming edges
	count := 0
	for _, node := range cg.Nodes {
		if node != nil && len(node.In) == 0 {
			count++
		}
	}
	return count
}
