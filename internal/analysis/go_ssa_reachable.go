package analysis

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nano-harness/impact-analyzer/internal/lang/golang/godeep"
)

// GoReachabilityProvider returns a set of file:line positions for Go functions/methods
// that are determined to be reachable by SSA+RTA analysis. Returns nil on failure.
type GoReachabilityProvider interface {
	Reachable() (map[PosKey]bool, error)
}

// PosKey represents a file:line:name position for symbol lookup
type PosKey struct {
	File string // Absolute path
	Line int    // 1-based line number
	Name string // Function/method name to disambiguate same-line declarations
}

// PosKeyOf creates a PosKey from a symbol's file, line and name
func PosKeyOf(file string, line int, name string) PosKey {
	return PosKey{File: file, Line: line, Name: name}
}

// GoDeepReachability implements GoReachabilityProvider using godeep SSA/RTA.
// It can operate standalone or delegate to a shared SSASession.
type GoDeepReachability struct {
	repoRoot          string
	customEntryPoints []string
	buildTags         []string
	session           *SSASession // optional shared session
}

// NewGoDeepReachability creates a new SSA-based reachability provider
func NewGoDeepReachability(repoRoot string, customEntryPoints, buildTags []string) GoReachabilityProvider {
	return &GoDeepReachability{
		repoRoot:          repoRoot,
		customEntryPoints: customEntryPoints,
		buildTags:         buildTags,
	}
}

// NewGoDeepReachabilityWithSession creates a reachability provider backed by a shared SSASession.
func NewGoDeepReachabilityWithSession(session *SSASession) GoReachabilityProvider {
	return &GoDeepReachability{session: session}
}

// Reachable performs SSA+RTA analysis and returns the set of reachable function positions.
// Returns (nil, err) on failure, allowing the caller to fall back to tree-sitter only.
func (g *GoDeepReachability) Reachable() (map[PosKey]bool, error) {
	// If we have a shared session, delegate to it
	if g.session != nil {
		return g.session.Reachable()
	}

	// Standalone mode: perform analysis directly
	// Verify repo has go.mod
	goModPath := filepath.Join(g.repoRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no go.mod found at %s", goModPath)
	}

	// Create analyzer with build tags
	analyzer := godeep.NewAnalyzer()
	if len(g.buildTags) > 0 {
		analyzer = analyzer.WithBuildTags(g.buildTags)
	}

	// Convert repoRoot to absolute path
	absRoot, err := filepath.Abs(g.repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Load all packages in the repository using absolute path
	pattern := absRoot + "/..."
	if err := analyzer.LoadPackages(pattern); err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}

	// Build SSA representation
	if err := analyzer.BuildSSA(); err != nil {
		return nil, fmt.Errorf("failed to build SSA: %w", err)
	}

	// Build RTA call graph with optional custom entry points
	cg, err := analyzer.BuildCallGraph(g.customEntryPoints...)
	if err != nil {
		return nil, fmt.Errorf("failed to build call graph: %w", err)
	}

	// Extract reachable function positions
	positions := analyzer.ReachableFunctionPositions(cg)

	// Convert godeep.Position to PosKey
	result := make(map[PosKey]bool, len(positions))
	for pos := range positions {
		result[PosKey{File: pos.File, Line: pos.Line, Name: pos.Name}] = true
	}

	return result, nil
}
