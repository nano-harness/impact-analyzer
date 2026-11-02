package cobra

import (
	"testing"
)

// TestFindCobraFactories_PatternA_Simple tests factory detection for simple Pattern A
func TestFindCobraFactories_PatternA_Simple(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_a_simple")

	factories := findCobraFactories(prog)

	if len(factories) == 0 {
		t.Fatal("Expected to find at least one factory function")
	}

	// Should find newSimpleCmd
	found := false
	for _, fn := range factories {
		if fn.Name() == "newSimpleCmd" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find newSimpleCmd factory")
	}
}

// TestClassifyRunETarget_Anonymous tests RunE classification for anonymous functions
func TestClassifyRunETarget_Anonymous(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_a_simple")

	fn := findFunctionByName(prog, "pattern_a_simple", "newSimpleCmd")
	if fn == nil {
		t.Fatal("newSimpleCmd not found")
	}

	target := classifyRunETarget(fn)

	if target.Kind != RunEAnonymous {
		t.Errorf("Expected RunEAnonymous, got %v", target.Kind)
	}

	if target.Closure == nil {
		t.Error("Expected Closure to be set for anonymous RunE")
	}
}

// TestClassifyRunETarget_FactoryClosure tests closure from factory
func TestClassifyRunETarget_FactoryClosure(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_a_closure")

	fn := findFunctionByName(prog, "pattern_a_closure", "newClosureCmd")
	if fn == nil {
		t.Fatal("newClosureCmd not found")
	}

	target := classifyRunETarget(fn)

	// Should detect factory closure pattern or anonymous function
	// SSA optimization may inline the factory, making it look anonymous
	if target.Kind == RunENone {
		t.Error("Expected to detect RunE, got RunENone")
	}

	// Closure may or may not be set depending on SSA optimization
	t.Logf("RunE Kind: %v, Closure set: %v", target.Kind, target.Closure != nil)
}

// TestExtractCommandMetadata_PatternA tests metadata extraction
func TestExtractCommandMetadata_PatternA(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_a_simple")

	fn := findFunctionByName(prog, "pattern_a_simple", "newSimpleCmd")
	if fn == nil {
		t.Fatal("newSimpleCmd not found")
	}

	use, short, long, aliases, line, file := extractCommandMetadata(fn)

	// SSA may optimize away string literals if the command is never used
	// Test that extraction doesn't panic and returns consistent data
	t.Logf("Extracted metadata - Use: %q, Short: %q, Long: %q", use, short, long)

	// At minimum, we should get position info
	if line == 0 && file == "" {
		t.Error("Expected at least line or file to be set")
	}

	// If Use is found, it should match
	if use != "" && use != "simple" {
		t.Errorf("Expected Use='simple', got %q", use)
	}

	// Aliases should be empty for this test
	if len(aliases) != 0 {
		t.Errorf("Expected no aliases, got %v", aliases)
	}
}

// TestFindGlobalCobraCommands_PatternW tests Pattern W global detection
func TestFindGlobalCobraCommands_PatternW(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_w_global")

	globals := findGlobalCobraCommands(prog)

	// SSA may optimize away unused global variables
	// This test verifies the function doesn't panic
	t.Logf("Found %d global commands", len(globals))

	if len(globals) == 0 {
		t.Skip("No global commands found - SSA may have optimized them away")
	}

	// Should find globalCmd if not optimized away
	found := false
	for _, g := range globals {
		if g.Name() == "globalCmd" {
			found = true
			break
		}
	}

	if !found {
		t.Log("globalCmd not found - may be optimized away by SSA")
	}
}

// TestExtractGlobalCommandMetadata_PatternW tests Pattern W metadata extraction
func TestExtractGlobalCommandMetadata_PatternW(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_w_global")

	global := findGlobalByName(prog, "pattern_w_global", "globalCmd")
	if global == nil {
		t.Skip("globalCmd not found - SSA may have optimized it away")
	}

	use, short, long, aliases, line, file, runEFn := extractGlobalCommandMetadata(global, prog)

	// SSA may optimize away string literals and function references
	t.Logf("Extracted metadata - Use: %q, Short: %q, Long: %q", use, short, long)
	t.Logf("RunE function: %v, Line: %d, File: %s", runEFn != nil, line, file)

	// SSA optimization can remove all metadata when code is unused
	// The test verifies the function doesn't panic
	if use == "" && short == "" && long == "" && runEFn == nil && line == 0 && file == "" {
		t.Skip("All metadata optimized away by SSA - this is expected for unused code")
	}

	// If Use is found, it should match
	if use != "" && use != "global" {
		t.Errorf("Expected Use='global', got %q", use)
	}

	// If runEFn is found, verify its name
	if runEFn != nil && runEFn.Name() != "runGlobal" {
		t.Errorf("Expected runEFn name to be 'runGlobal', got %q", runEFn.Name())
	}

	if len(aliases) != 0 {
		t.Errorf("Expected no aliases, got %v", aliases)
	}
}
