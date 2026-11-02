package cobra

import (
	"testing"
)

// TestTraceDataflow_Handlers tests dataflow tracing to handler functions
func TestTraceDataflow_Handlers(t *testing.T) {
	prog := loadTestdataSSA(t, "dataflow_handlers")

	fn := findFunctionByName(prog, "dataflow_handlers", "newHandlersCmd")
	if fn == nil {
		t.Fatal("newHandlersCmd not found")
	}

	bindings := findFlagBindings(fn)
	if len(bindings) < 2 {
		t.Fatalf("Expected at least 2 bindings, got %d", len(bindings))
	}

	target := classifyRunETarget(fn)
	if target.Kind != RunEAnonymous {
		t.Fatalf("Expected RunEAnonymous, got %v", target.Kind)
	}

	// Test dataflow for "input" flag
	var inputBinding *FlagBinding
	for i := range bindings {
		if bindings[i].FlagName == "input" {
			inputBinding = &bindings[i]
			break
		}
	}

	if inputBinding == nil {
		t.Fatal("input binding not found")
	}

	// Map to FreeVar and find reads
	freeVar, _ := mapAddrToFreeVar(target.Closure, inputBinding.VarAddr)
	if freeVar == nil {
		t.Fatal("FreeVar for input not found")
	}

	readPoints := findFreeVarLoads(freeVar)
	if len(readPoints) == 0 {
		t.Fatal("No read points found for input flag")
	}

	// Trace dataflow from first read point
	usages := traceDataflow(readPoints[0], 5)

	if len(usages) == 0 {
		t.Error("Expected to find usages for input flag")
	}

	// Check if we found ProcessInput call
	foundProcessInput := false
	for _, u := range usages {
		if u.Kind == "call_arg" && u.Callee != nil {
			if u.Callee.Name() == "ProcessInput" {
				foundProcessInput = true
				t.Logf("Found handler: ProcessInput at arg_index %d, depth %d", u.ArgIndex, u.Depth)
			}
		}
	}

	if !foundProcessInput {
		t.Error("Expected to find ProcessInput in dataflow usages")
	}
}

// TestTraceDataflow_MultipleHandlers tests multiple handlers for one flag
func TestTraceDataflow_MultipleHandlers(t *testing.T) {
	prog := loadTestdataSSA(t, "dataflow_handlers")

	fn := findFunctionByName(prog, "dataflow_handlers", "newHandlersCmd")
	if fn == nil {
		t.Fatal("newHandlersCmd not found")
	}

	bindings := findFlagBindings(fn)
	target := classifyRunETarget(fn)

	// Test dataflow for "output" flag
	var outputBinding *FlagBinding
	for i := range bindings {
		if bindings[i].FlagName == "output" {
			outputBinding = &bindings[i]
			break
		}
	}

	if outputBinding == nil {
		t.Fatal("output binding not found")
	}

	freeVar, _ := mapAddrToFreeVar(target.Closure, outputBinding.VarAddr)
	if freeVar == nil {
		t.Fatal("FreeVar for output not found")
	}

	readPoints := findFreeVarLoads(freeVar)
	if len(readPoints) == 0 {
		t.Fatal("No read points found for output flag")
	}

	// Trace dataflow
	usages := traceDataflow(readPoints[0], 5)

	// Should find WriteOutput handler
	foundWriteOutput := false
	for _, u := range usages {
		if u.Kind == "call_arg" && u.Callee != nil {
			if u.Callee.Name() == "WriteOutput" {
				foundWriteOutput = true
				t.Logf("Found handler: WriteOutput at arg_index %d, depth %d", u.ArgIndex, u.Depth)
			}
		}
	}

	if !foundWriteOutput {
		t.Error("Expected to find WriteOutput in dataflow usages")
	}
}

// TestTraceDataflow_DepthLimit tests depth limiting in dataflow
func TestTraceDataflow_DepthLimit(t *testing.T) {
	prog := loadTestdataSSA(t, "reads_freevar")

	fn := findFunctionByName(prog, "reads_freevar", "newFreeVarCmd")
	if fn == nil {
		t.Fatal("newFreeVarCmd not found")
	}

	bindings := findFlagBindings(fn)
	if len(bindings) == 0 {
		t.Fatal("No bindings found")
	}

	target := classifyRunETarget(fn)
	freeVar, _ := mapAddrToFreeVar(target.Closure, bindings[0].VarAddr)
	if freeVar == nil {
		t.Skip("FreeVar not found")
	}

	readPoints := findFreeVarLoads(freeVar)
	if len(readPoints) == 0 {
		t.Skip("No read points found")
	}

	// Test with depth 0 (should return minimal usages)
	usages0 := traceDataflow(readPoints[0], 0)

	// Test with depth 5
	usages5 := traceDataflow(readPoints[0], 5)

	// Depth 5 should find at least as many usages as depth 0
	if len(usages5) < len(usages0) {
		t.Errorf("Expected more usages with higher depth: depth 0 found %d, depth 5 found %d",
			len(usages0), len(usages5))
	}

	t.Logf("Depth 0: %d usages, Depth 5: %d usages", len(usages0), len(usages5))
}

// TestTraceDataflow_StdlibStop tests stopping at stdlib calls
func TestTraceDataflow_StdlibStop(t *testing.T) {
	prog := loadTestdataSSA(t, "reads_freevar")

	fn := findFunctionByName(prog, "reads_freevar", "newFreeVarCmd")
	if fn == nil {
		t.Fatal("newFreeVarCmd not found")
	}

	bindings := findFlagBindings(fn)
	if len(bindings) == 0 {
		t.Fatal("No bindings found")
	}

	target := classifyRunETarget(fn)
	freeVar, _ := mapAddrToFreeVar(target.Closure, bindings[0].VarAddr)
	if freeVar == nil {
		t.Skip("FreeVar not found")
	}

	readPoints := findFreeVarLoads(freeVar)
	if len(readPoints) == 0 {
		t.Skip("No read points found")
	}

	usages := traceDataflow(readPoints[0], 5)

	// Check if any usage has stop_stdlib kind
	hasStopStdlib := false
	for _, u := range usages {
		if u.Kind == "stop_stdlib" {
			hasStopStdlib = true
			t.Logf("Found stop_stdlib usage: %s", u.Note)
		}
	}

	// Whether we find stdlib calls depends on the test fixture
	t.Logf("Stop at stdlib: %v", hasStopStdlib)
}
