package godeep

import (
	"go/types"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// TestAnalyzer_OwnerField verifies that Owner is correctly set for methods
func TestAnalyzer_OwnerField(t *testing.T) {
	// Create a test package with methods
	testPackage := `
package testpkg

type Server struct{}

func (s *Server) Start() {}
func (s Server) Stop() {}

type Client[T any] struct{}
func (c *Client[T]) Connect() {}

func PlainFunc() {}
`
	_ = testPackage // Will use in integration test

	// Test deriveReceiverTypeName directly
	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		{
			name:     "pointer receiver",
			typ:      types.NewPointer(types.NewNamed(types.NewTypeName(0, nil, "Server", nil), types.NewStruct(nil, nil), nil)),
			expected: "Server",
		},
		{
			name:     "value receiver",
			typ:      types.NewNamed(types.NewTypeName(0, nil, "Client", nil), types.NewStruct(nil, nil), nil),
			expected: "Client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveReceiverTypeName(tt.typ)
			if result != tt.expected {
				t.Errorf("deriveReceiverTypeName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAnalyzer_WithBuildTags verifies build tags configuration
func TestAnalyzer_WithBuildTags(t *testing.T) {
	analyzer := NewAnalyzer()

	tags := []string{"integration", "e2e"}
	result := analyzer.WithBuildTags(tags)

	if result != analyzer {
		t.Error("WithBuildTags should return the same analyzer for chaining")
	}

	if len(analyzer.buildTags) != 2 {
		t.Errorf("Expected 2 build tags, got %d", len(analyzer.buildTags))
	}

	if analyzer.buildTags[0] != "integration" || analyzer.buildTags[1] != "e2e" {
		t.Errorf("Build tags not set correctly: %v", analyzer.buildTags)
	}
}

// TestAnalyzer_JoinTags verifies tag joining logic
func TestJoinTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "single tag",
			tags:     []string{"integration"},
			expected: "integration",
		},
		{
			name:     "multiple tags",
			tags:     []string{"integration", "e2e", "slow"},
			expected: "integration,e2e,slow",
		},
		{
			name:     "empty tags",
			tags:     []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinTags(tt.tags)
			if result != tt.expected {
				t.Errorf("joinTags() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAnalyzer_ReachableFunctionPositions verifies position extraction
func TestAnalyzer_ReachableFunctionPositions(t *testing.T) {
	analyzer := NewAnalyzer()

	// Load fmt package to test with real SSA
	err := analyzer.LoadPackages("fmt")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	err = analyzer.BuildSSA()
	if err != nil {
		t.Fatalf("Failed to build SSA: %v", err)
	}

	cg, err := analyzer.BuildCallGraph()
	if err != nil {
		t.Fatalf("Failed to build call graph: %v", err)
	}

	positions := analyzer.ReachableFunctionPositions(cg)

	if len(positions) == 0 {
		t.Error("Expected some reachable function positions")
	}

	// Verify positions have valid file and line
	for pos := range positions {
		if pos.File == "" {
			t.Error("Position has empty file")
		}
		if pos.Line <= 0 {
			t.Errorf("Position has invalid line: %d", pos.Line)
		}
	}
}

// TestAnalyzer_FunctionToSymbol_WithOwner verifies Owner field is set correctly
func TestAnalyzer_FunctionToSymbol_WithOwner(t *testing.T) {
	analyzer := NewAnalyzer()

	// Load a package that has methods
	err := analyzer.LoadPackages("fmt")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	err = analyzer.BuildSSA()
	if err != nil {
		t.Fatalf("Failed to build SSA: %v", err)
	}

	cg, err := analyzer.BuildCallGraph()
	if err != nil {
		t.Fatalf("Failed to build call graph: %v", err)
	}

	symbols := analyzer.ConvertToModel(cg)

	// Find a method symbol
	var foundMethod *model.Symbol
	for _, sym := range symbols {
		if sym.Kind == model.SymbolKindMethod {
			foundMethod = sym
			break
		}
	}

	if foundMethod != nil {
		// Verify that methods have Owner set
		if foundMethod.Owner == "" {
			t.Error("Method symbol should have Owner field set")
		}
	}

	// Verify functions don't have Owner
	for _, sym := range symbols {
		if sym.Kind == model.SymbolKindFunction && sym.Owner != "" {
			t.Errorf("Function %s should not have Owner, but has %q", sym.Name, sym.Owner)
		}
	}
}
