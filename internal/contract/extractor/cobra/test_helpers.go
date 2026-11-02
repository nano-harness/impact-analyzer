package cobra

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// loadTestdataSSA loads SSA for a testdata package
func loadTestdataSSA(t *testing.T, scenario string) *ssa.Program {
	t.Helper()

	// Get testdata directory
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get testdata dir: %v", err)
	}

	pkgPath := filepath.Join(testdataDir, scenario)

	// Load package
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
		Dir: pkgPath,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatalf("Failed to load package %s: %v", scenario, err)
	}

	if len(pkgs) == 0 {
		t.Fatalf("No packages loaded for %s", scenario)
	}

	if packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("Package %s has errors", scenario)
	}

	// Build SSA
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	return prog
}

// findFunctionByName finds a function by name in the program
func findFunctionByName(prog *ssa.Program, pkgPath, funcName string) *ssa.Function {
	for _, pkg := range prog.AllPackages() {
		if pkg == nil {
			continue
		}

		// Match by package path suffix (testdata packages have full paths)
		if pkgPath != "" && pkg.Pkg != nil && !endsWithPath(pkg.Pkg.Path(), pkgPath) {
			continue
		}

		for _, mem := range pkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok || fn == nil {
				continue
			}

			if fn.Name() == funcName {
				return fn
			}
		}
	}

	return nil
}

// findGlobalByName finds a global variable by name in the program
func findGlobalByName(prog *ssa.Program, pkgPath, varName string) *ssa.Global {
	for _, pkg := range prog.AllPackages() {
		if pkg == nil {
			continue
		}

		if pkgPath != "" && pkg.Pkg != nil && !endsWithPath(pkg.Pkg.Path(), pkgPath) {
			continue
		}

		for _, mem := range pkg.Members {
			g, ok := mem.(*ssa.Global)
			if !ok || g == nil {
				continue
			}

			if g.Name() == varName {
				return g
			}
		}
	}

	return nil
}

// endsWithPath checks if path ends with suffix (handles testdata relative paths)
func endsWithPath(path, suffix string) bool {
	return filepath.Base(path) == filepath.Base(suffix)
}

// assertContains checks if a slice contains a value
func assertContains(t *testing.T, slice []string, value string) {
	t.Helper()
	for _, v := range slice {
		if v == value {
			return
		}
	}
	t.Errorf("Expected slice to contain %q, but it doesn't. Slice: %v", value, slice)
}

// assertNotEmpty checks if a slice is not empty
func assertNotEmpty(t *testing.T, slice interface{}, name string) {
	t.Helper()
	switch s := slice.(type) {
	case []FlagBinding:
		if len(s) == 0 {
			t.Errorf("Expected %s to be non-empty", name)
		}
	case []Usage:
		if len(s) == 0 {
			t.Errorf("Expected %s to be non-empty", name)
		}
	case []*ssa.Function:
		if len(s) == 0 {
			t.Errorf("Expected %s to be non-empty", name)
		}
	default:
		t.Errorf("assertNotEmpty: unsupported type %T", slice)
	}
}
