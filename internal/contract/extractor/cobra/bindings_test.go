package cobra

import (
	"testing"
)

// TestFindFlagBindings_PatternA tests flag binding detection
func TestFindFlagBindings_PatternA(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_a_simple")

	fn := findFunctionByName(prog, "pattern_a_simple", "newSimpleCmd")
	if fn == nil {
		t.Fatal("newSimpleCmd not found")
	}

	bindings := findFlagBindings(fn)

	if len(bindings) < 2 {
		t.Fatalf("Expected at least 2 bindings, got %d", len(bindings))
	}

	// Check we found the flags
	flagNames := make(map[string]bool)
	for _, b := range bindings {
		flagNames[b.FlagName] = true
	}

	if !flagNames["text"] {
		t.Error("Expected to find 'text' flag")
	}

	if !flagNames["count"] {
		t.Error("Expected to find 'count' flag")
	}
}

// TestFindFlagBindings_MultipleTypes tests multiple flag types
func TestFindFlagBindings_MultipleTypes(t *testing.T) {
	prog := loadTestdataSSA(t, "bindings_complex")

	fn := findFunctionByName(prog, "bindings_complex", "newComplexBindingsCmd")
	if fn == nil {
		t.Fatal("newComplexBindingsCmd not found")
	}

	bindings := findFlagBindings(fn)

	if len(bindings) < 4 {
		t.Fatalf("Expected at least 4 bindings, got %d", len(bindings))
	}

	// Check flag types
	flagTypes := make(map[string]string)
	for _, b := range bindings {
		if b.FlagName != "" {
			flagTypes[b.FlagName] = b.FlagType
		}
	}

	// Note: SSA uses pflag method names as types (String, Int, Bool, StringSlice)
	// These are more precise than Go type names
	expectedTypes := map[string]string{
		"str":   "String",
		"int":   "Int",
		"bool":  "Bool",
		"slice": "StringSlice",
	}

	for flag, expectedType := range expectedTypes {
		actualType, found := flagTypes[flag]
		if !found {
			t.Errorf("Flag %q not found in bindings", flag)
			continue
		}

		if actualType != expectedType {
			t.Errorf("Flag %q: expected type %q, got %q", flag, expectedType, actualType)
		}
	}
}

// TestResolveBindingOwner_Factory tests owner resolution for factory commands
func TestResolveBindingOwner_Factory(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_a_simple")

	fn := findFunctionByName(prog, "pattern_a_simple", "newSimpleCmd")
	if fn == nil {
		t.Fatal("newSimpleCmd not found")
	}

	bindings := findFlagBindings(fn)
	if len(bindings) == 0 {
		t.Fatal("No bindings found")
	}

	// Check that bindings have owner info
	for _, b := range bindings {
		if b.Owner.Kind == "" {
			t.Errorf("Binding for flag %q has empty Owner.Kind", b.FlagName)
		}

		// For Pattern A, owner should be factory
		if b.Owner.Kind == "factory" && b.Owner.Factory == nil {
			t.Errorf("Binding for flag %q has Kind='factory' but Factory is nil", b.FlagName)
		}
	}
}

// TestFindFlagBindingsForGlobal_PatternW tests global command bindings
func TestFindFlagBindingsForGlobal_PatternW(t *testing.T) {
	prog := loadTestdataSSA(t, "pattern_w_global")

	global := findGlobalByName(prog, "pattern_w_global", "globalCmd")
	if global == nil {
		t.Fatal("globalCmd not found")
	}

	bindings := findFlagBindingsForGlobal(prog, global)

	if len(bindings) < 1 {
		t.Fatalf("Expected at least 1 binding, got %d", len(bindings))
	}

	// Should find the "option" flag
	found := false
	for _, b := range bindings {
		if b.FlagName == "option" {
			found = true

			// Check owner is global
			if b.Owner.Kind != "global" {
				t.Errorf("Expected Owner.Kind='global', got %q", b.Owner.Kind)
			}

			if b.Owner.Global == nil {
				t.Error("Expected Owner.Global to be set")
			}

			break
		}
	}

	if !found {
		t.Error("Expected to find 'option' flag")
	}
}

// TestBindingOwner_Unknown tests unknown owner handling
func TestBindingOwner_Unknown(t *testing.T) {
	// resolveBindingOwner should return Kind="unknown" for nil input
	owner := resolveBindingOwner(nil)

	if owner.Kind != "unknown" {
		t.Errorf("Expected Kind='unknown' for nil call, got %q", owner.Kind)
	}

	if owner.Factory != nil {
		t.Error("Expected Factory to be nil for unknown owner")
	}

	if owner.Global != nil {
		t.Error("Expected Global to be nil for unknown owner")
	}
}
