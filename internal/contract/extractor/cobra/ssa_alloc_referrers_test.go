package cobra

import "testing"

func TestExtractCommandMetadata_CompositeLiteralFactory(t *testing.T) {
	prog := loadTestdataSSA(t, "composite_literal_factory")
	fn := findFunctionByName(prog, "composite_literal_factory", "newCmd")
	if fn == nil {
		t.Fatal("newCmd not found")
	}

	use, short, long, aliases, _, _ := extractCommandMetadata(fn)
	if use != "deploy <project-dir>" {
		t.Fatalf("Use = %q, want %q", use, "deploy <project-dir>")
	}
	if short != "Deploy packages" {
		t.Fatalf("Short = %q, want %q", short, "Deploy packages")
	}
	if long != "Deploy Maven packages from a local project" {
		t.Fatalf("Long = %q, want %q", long, "Deploy Maven packages from a local project")
	}
	assertContains(t, aliases, "d")
	assertContains(t, aliases, "dep")
}

func TestExtractCommandMetadata_FactoryWithFlagBindings(t *testing.T) {
	prog := loadTestdataSSA(t, "factory_with_flags")
	fn := findFunctionByName(prog, "factory_with_flags", "newCmd")
	if fn == nil {
		t.Fatal("newCmd not found")
	}

	use, short, _, _, _, _ := extractCommandMetadata(fn)
	if use != "deploy <project-dir>" {
		t.Fatalf("Use = %q, want %q", use, "deploy <project-dir>")
	}
	if short != "Deploy packages" {
		t.Fatalf("Short = %q, want %q", short, "Deploy packages")
	}

	bindings := findFlagBindings(fn)
	if len(bindings) != 2 {
		t.Fatalf("got %d flag bindings, want 2", len(bindings))
	}
	flagTypes := map[string]string{}
	for _, b := range bindings {
		flagTypes[b.FlagName] = b.FlagType
	}
	if flagTypes["mvn-args"] != "StringArray" {
		t.Fatalf("mvn-args type = %q, want StringArray", flagTypes["mvn-args"])
	}
	if flagTypes["use-mvn-scan"] != "Bool" {
		t.Fatalf("use-mvn-scan type = %q, want Bool", flagTypes["use-mvn-scan"])
	}
}

func TestExtractCommandMetadata_TwoFactoriesInOneFile(t *testing.T) {
	prog := loadTestdataSSA(t, "two_factories")
	first := findFunctionByName(prog, "two_factories", "newFirstCmd")
	second := findFunctionByName(prog, "two_factories", "newSecondCmd")
	if first == nil || second == nil {
		t.Fatalf("expected both factories, got first=%v second=%v", first != nil, second != nil)
	}

	firstUse, firstShort, _, _, _, _ := extractCommandMetadata(first)
	secondUse, secondShort, _, _, _, _ := extractCommandMetadata(second)
	if firstUse != "first" || firstShort != "First command" {
		t.Fatalf("first metadata = (%q, %q), want (first, First command)", firstUse, firstShort)
	}
	if secondUse != "second" || secondShort != "Second command" {
		t.Fatalf("second metadata = (%q, %q), want (second, Second command)", secondUse, secondShort)
	}

	if got := len(findFlagBindings(first)); got != 1 {
		t.Fatalf("first flag bindings = %d, want 1", got)
	}
	if got := len(findFlagBindings(second)); got != 0 {
		t.Fatalf("second flag bindings = %d, want 0", got)
	}
}
