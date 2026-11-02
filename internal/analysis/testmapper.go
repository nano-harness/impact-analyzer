package analysis

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// TestMapper maps code changes to affected tests
type TestMapper struct {
	callGraph *graph.CallGraph
	repoRoot  string

	// testFiles, when non-nil, replaces the repository walk in the
	// collectFromTestFiles fallback (injected by the orchestrator, which
	// already listed test files during call-graph construction).
	testFiles []string
	// parseFile and extractCalls, when set, replace direct adapter calls in
	// the fallback path (injected by the orchestrator to route through its
	// per-run parse cache).
	parseFile    func(path string) (*model.FileSymbols, error)
	extractCalls func(path string) ([]*model.CallSite, error)
}

// TestMapperOption customizes a TestMapper at construction time.
type TestMapperOption func(*TestMapper)

// WithTestFiles supplies a pre-collected test file list, skipping the
// repository walk in the fallback pass.
func WithTestFiles(files []string) TestMapperOption {
	return func(tm *TestMapper) {
		tm.testFiles = files
	}
}

// WithParseFuncs injects parse/extract functions used instead of direct
// adapter.ParseFile/adapter.ExtractCalls calls in the fallback pass (e.g.
// backed by the orchestrator's per-run parse cache).
func WithParseFuncs(parse func(path string) (*model.FileSymbols, error), calls func(path string) ([]*model.CallSite, error)) TestMapperOption {
	return func(tm *TestMapper) {
		tm.parseFile = parse
		tm.extractCalls = calls
	}
}

// NewTestMapper creates a new test mapper
func NewTestMapper(cg *graph.CallGraph, repoRoot string, opts ...TestMapperOption) *TestMapper {
	tm := &TestMapper{
		callGraph: cg,
		repoRoot:  repoRoot,
	}
	for _, opt := range opts {
		opt(tm)
	}
	return tm
}

// FindAffectedTests finds tests affected by the given changes using forward logic:
// Prefer call graph data (test symbols' callees) to avoid re-parsing test files;
// fall back to walking and parsing test files when the graph has no test symbols
// indexed (e.g. file-mode analysis does not build a test-file graph).
func (tm *TestMapper) FindAffectedTests(changes []*model.Change) ([]*model.Symbol, error) {
	// Build target sets: qualified names and bare names of changed symbols
	targetQNames := make(map[string]bool)
	targetByQName := make(map[string]*model.Symbol)
	targetsByName := make(map[string][]*model.Symbol)
	for _, change := range changes {
		if change.Symbol != nil {
			targetQNames[change.Symbol.QualifiedName()] = true
			targetByQName[change.Symbol.QualifiedName()] = change.Symbol
			targetsByName[change.Symbol.Name] = append(targetsByName[change.Symbol.Name], change.Symbol)
		}
	}

	if len(targetQNames) == 0 {
		return nil, nil
	}

	// Track affected test symbols
	affectedTests := make(map[string]*model.Symbol)

	// Forward pass: for each test symbol in the call graph, check whether any
	// of its callees intersects the changed target set.
	if tm.hasTestSymbols() {
		tm.collectFromCallGraph(targetByQName, targetsByName, affectedTests)
	} else {
		// Fallback: walk all test files, parse them, check if they call any
		// changed symbols.
		if err := tm.collectFromTestFiles(targetQNames, targetsByName, affectedTests); err != nil {
			return nil, err
		}
	}

	// Also check reverse BFS as a supplement (for already-indexed tests)
	// This catches cases where test files were already in the callgraph
	for _, change := range changes {
		if change.Symbol == nil {
			continue
		}

		allCallers := tm.callGraph.GetAllCallers(change.Symbol, -1)
		for _, caller := range allCallers {
			if tm.isTestSymbol(caller) {
				key := caller.QualifiedName() + "@" + caller.File
				affectedTests[key] = caller
			}
		}
	}

	// Convert map to slice; sort for deterministic output (map iteration
	// order otherwise leaks into affected_tests and flaps run to run).
	tests := make([]*model.Symbol, 0, len(affectedTests))
	for _, test := range affectedTests {
		tests = append(tests, test)
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].File != tests[j].File {
			return tests[i].File < tests[j].File
		}
		return tests[i].QualifiedName() < tests[j].QualifiedName()
	})

	return tests, nil
}

// hasTestSymbols reports whether the call graph has any test symbols indexed.
func (tm *TestMapper) hasTestSymbols() bool {
	if tm.callGraph == nil {
		return false
	}
	for _, sym := range tm.callGraph.Symbols() {
		if sym != nil && sym.IsTest {
			return true
		}
	}
	return false
}

// collectFromCallGraph maps test symbols to changes using the call graph's
// caller->callee edges, reusing symbols already parsed during graph build.
// Every match is validated against the test symbol's package context: callee
// resolution may stem from a bare-name edge (e.g. single-candidate package
// guess), which would otherwise flag any test calling a same-named function
// in another package.
func (tm *TestMapper) collectFromCallGraph(targetByQName map[string]*model.Symbol, targetsByName map[string][]*model.Symbol, affectedTests map[string]*model.Symbol) {
	for _, sym := range tm.callGraph.Symbols() {
		if sym == nil || !sym.IsTest {
			continue
		}
		var imports []string
		importsReady := false
		for _, callee := range tm.callGraph.GetCallees(sym) {
			if callee == nil {
				continue
			}
			matched := false
			if target, ok := targetByQName[callee.QualifiedName()]; ok {
				if !importsReady {
					imports = tm.fileImports(sym.File)
					importsReady = true
				}
				matched = tm.testContextMatches(sym, target, imports)
			} else if targets, ok := targetsByName[callee.Name]; ok {
				// Bare-name hits need package context validation to avoid
				// cross-package false positives on same-named functions.
				matched = tm.bareNameMatchValid(sym, callee, nil, targets)
			}
			if matched {
				key := sym.QualifiedName() + "@" + sym.File
				affectedTests[key] = sym
				break
			}
		}
	}
}

// collectFromTestFiles is the fallback forward pass: it walks the repository,
// parses every test file, and maps the test function enclosing each call that
// targets a changed symbol.
func (tm *TestMapper) collectFromTestFiles(targetQNames map[string]bool, targetsByName map[string][]*model.Symbol, affectedTests map[string]*model.Symbol) error {
	// Walk all test files in the repository (or use the injected list)
	testFiles := tm.testFiles
	if testFiles == nil {
		var err error
		testFiles, err = tm.listTestFiles()
		if err != nil {
			return err
		}
	}

	// For each test file, parse and check if it calls any changed symbols
	for _, testFile := range testFiles {
		// Parse test file to get symbols
		fileSymbols, err := tm.parse(testFile)
		if err != nil {
			continue
		}

		// Extract calls from test file
		calls, err := tm.calls(testFile)
		if err != nil {
			continue
		}

		// Check each call to see if it targets a changed symbol
		for _, call := range calls {
			if call.Callee == nil {
				continue
			}

			// Check if this call targets a changed symbol
			calleeQName := call.Callee.QualifiedName()
			calleeName := call.Callee.Name

			matched := targetQNames[calleeQName]
			if !matched {
				// Bare-name hits need package context validation to avoid
				// cross-package false positives on same-named functions.
				if targets, ok := targetsByName[calleeName]; ok {
					if testFn := tm.findEnclosingTestFunction(fileSymbols.Symbols, call.Line); testFn != nil {
						matched = tm.bareNameMatchValid(testFn, call.Callee, fileSymbols.Imports, targets)
					}
				}
			}
			if !matched {
				continue
			}

			// Find which test function contains this call
			testFn := tm.findEnclosingTestFunction(fileSymbols.Symbols, call.Line)
			if testFn != nil {
				// Use qualified name + file path as unique key
				key := testFn.QualifiedName() + "@" + testFile
				affectedTests[key] = testFn
			}
		}
	}
	return nil
}

// parse parses a test file, via the injected function when set, otherwise via
// the language adapter directly.
func (tm *TestMapper) parse(path string) (*model.FileSymbols, error) {
	if tm.parseFile != nil {
		return tm.parseFile(path)
	}
	adapter, err := tm.adapterFor(path)
	if err != nil {
		return nil, err
	}
	return adapter.ParseFile(path)
}

// calls extracts call sites from a test file, via the injected function when
// set, otherwise via the language adapter directly.
func (tm *TestMapper) calls(path string) ([]*model.CallSite, error) {
	if tm.extractCalls != nil {
		return tm.extractCalls(path)
	}
	adapter, err := tm.adapterFor(path)
	if err != nil {
		return nil, err
	}
	return adapter.ExtractCalls(path)
}

func (tm *TestMapper) adapterFor(path string) (lang.LanguageAdapter, error) {
	language := lang.DetectLanguage(path)
	if language == model.LanguageUnknown {
		return nil, fmt.Errorf("unknown language for %s", path)
	}
	return lang.Get(language)
}

// bareNameMatchValid validates a bare-name match between a test-side callee
// and same-named changed targets using package context, mirroring the
// semantics of the call graph's filterCallersByContext: a resolved callee
// must share the target's package or file; an unresolved callee is kept only
// when the test-side context matches the target. Targets without package
// info cannot be validated and are kept (recall-preserving).
func (tm *TestMapper) bareNameMatchValid(testSym, callee *model.Symbol, imports []string, targets []*model.Symbol) bool {
	for _, target := range targets {
		if target.Package == "" {
			return true
		}
		// Resolved callee carrying package info: require same package or file.
		if callee.Package != "" {
			if callee.Package == target.Package || (callee.File != "" && callee.File == target.File) {
				return true
			}
			continue
		}
		// Unresolved callee: fall back to test-side context.
		if tm.testContextMatches(testSym, target, imports) {
			return true
		}
	}
	return false
}

// testContextMatches reports whether a test symbol plausibly calls the target
// based on package context alone: same package, same file, or the test file
// imports the target package (Go: full path or trailing element; Java: class
// import under the target package). Targets without package info cannot be
// validated and match (recall-preserving).
func (tm *TestMapper) testContextMatches(testSym, target *model.Symbol, imports []string) bool {
	if target == nil || target.Package == "" {
		return true
	}
	if testSym.Package == target.Package || (target.File != "" && testSym.File == target.File) {
		return true
	}
	if imports == nil {
		imports = tm.fileImports(testSym.File)
	}
	for _, imp := range imports {
		if imp == target.Package ||
			strings.HasSuffix(imp, "/"+target.Package) ||
			strings.HasPrefix(imp, target.Package+".") {
			return true
		}
	}
	return false
}

// fileImports returns the import paths declared in a file, best-effort.
func (tm *TestMapper) fileImports(file string) []string {
	language := lang.DetectLanguage(file)
	if language == model.LanguageUnknown {
		return nil
	}
	adapter, err := lang.Get(language)
	if err != nil {
		return nil
	}
	imports, err := adapter.ResolveImports(file)
	if err != nil {
		return nil
	}
	return imports
}

// findEnclosingTestFunction finds the test function that contains the given line
func (tm *TestMapper) findEnclosingTestFunction(symbols []*model.Symbol, line int) *model.Symbol {
	for _, sym := range symbols {
		// Check if this is a test symbol
		if !sym.IsTest {
			continue
		}

		// Check if line is within symbol bounds
		endLine := sym.EndLine
		if endLine == 0 {
			// Fallback: if EndLine not set, use simple heuristic
			if line >= sym.Line {
				return sym
			}
		} else {
			// Proper containment check
			if line >= sym.Line && line <= endLine {
				return sym
			}
		}
	}
	return nil
}

// listTestFiles walks the repository and returns all test file paths
func (tm *TestMapper) listTestFiles() ([]string, error) {
	var testFiles []string

	err := filepath.WalkDir(tm.repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip common non-source directories
			switch d.Name() {
			case ".git", "node_modules", "vendor", "dist", "build", ".impact", "target":
				return filepath.SkipDir
			}
			return nil
		}

		language := lang.DetectLanguage(path)
		if language == model.LanguageUnknown {
			return nil
		}

		adapter, err := lang.Get(language)
		if err != nil {
			return nil
		}

		if adapter.IsTestFile(path) {
			testFiles = append(testFiles, path)
		}

		return nil
	})

	return testFiles, err
}

func (tm *TestMapper) isTestSymbol(symbol *model.Symbol) bool {
	if symbol.IsTest {
		return true
	}

	// Check if file is a test file based on language conventions
	language := lang.DetectLanguage(symbol.File)
	adapter, err := lang.Get(language)
	if err != nil {
		return false
	}

	return adapter.IsTestFile(symbol.File)
}

// IsTestFile determines if a file path is a test file
func IsTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)

	switch ext {
	case ".go":
		return filepath.Ext(base[:len(base)-len(ext)]) == "_test"
	case ".java":
		name := base[:len(base)-len(ext)]
		return len(name) > 4 && (name[len(name)-4:] == "Test" || name[len(name)-5:] == "Tests")
	case ".js", ".ts", ".jsx", ".tsx":
		name := base[:len(base)-len(ext)]
		return len(name) > 5 && (name[len(name)-5:] == ".test" || name[len(name)-5:] == ".spec")
	}

	return false
}
