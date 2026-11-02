package graph

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestGetAllCallers_Basic(t *testing.T) {
	// Build a simple call graph: A <- B <- C <- D
	// (D calls C, C calls B, B calls A)
	cg := NewCallGraph()

	a := &model.Symbol{Name: "A", Package: "pkg"}
	b := &model.Symbol{Name: "B", Package: "pkg"}
	c := &model.Symbol{Name: "C", Package: "pkg"}
	d := &model.Symbol{Name: "D", Package: "pkg"}

	cg.AddEdge(b, a) // B calls A
	cg.AddEdge(c, b) // C calls B
	cg.AddEdge(d, c) // D calls C

	// Get all callers of A
	callers := cg.GetAllCallers(a, -1)

	// Should get B, C, D (all transitive callers)
	if len(callers) != 3 {
		t.Fatalf("Expected 3 callers, got %d", len(callers))
	}

	// Verify callers contain B, C, D
	callerNames := make(map[string]bool)
	for _, caller := range callers {
		callerNames[caller.Name] = true
	}

	if !callerNames["B"] || !callerNames["C"] || !callerNames["D"] {
		t.Errorf("Expected callers B, C, D, got %v", callerNames)
	}
}

func TestGetAllCallersWithStop_NoStopFn(t *testing.T) {
	// Build call graph: A <- B <- C <- D
	cg := NewCallGraph()

	a := &model.Symbol{Name: "A", Package: "pkg"}
	b := &model.Symbol{Name: "B", Package: "pkg"}
	c := &model.Symbol{Name: "C", Package: "pkg"}
	d := &model.Symbol{Name: "D", Package: "pkg"}

	cg.AddEdge(b, a)
	cg.AddEdge(c, b)
	cg.AddEdge(d, c)

	// GetAllCallersWithStop with nil stopFn should behave like GetAllCallers
	callersWithNil := cg.GetAllCallersWithStop(a, -1, nil)
	callersNormal := cg.GetAllCallers(a, -1)

	if len(callersWithNil) != len(callersNormal) {
		t.Errorf("GetAllCallersWithStop(nil) returned %d callers, GetAllCallers returned %d",
			len(callersWithNil), len(callersNormal))
	}
}

func TestGetAllCallersWithStop_EntryPointBoundary(t *testing.T) {
	// Build call graph: A <- B <- C <- D
	// Stop at C (C is entry point)
	cg := NewCallGraph()

	a := &model.Symbol{Name: "A", Package: "pkg"}
	b := &model.Symbol{Name: "B", Package: "pkg"}
	c := &model.Symbol{Name: "C", Package: "pkg"}
	d := &model.Symbol{Name: "D", Package: "pkg"}

	cg.AddEdge(b, a) // B calls A
	cg.AddEdge(c, b) // C calls B
	cg.AddEdge(d, c) // D calls C

	// Stop at C - NEW BEHAVIOR: should get B and C (C itself IS collected)
	stopFn := func(s *model.Symbol) bool {
		return s.Name == "C"
	}

	callers := cg.GetAllCallersWithStop(a, -1, stopFn)

	// Should get B and C (C is collected, D is not traversed)
	if len(callers) != 2 {
		t.Fatalf("Expected 2 callers (B, C), got %d: %v", len(callers), callers)
	}

	callerNames := make(map[string]bool)
	for _, caller := range callers {
		callerNames[caller.Name] = true
	}

	if !callerNames["B"] || !callerNames["C"] {
		t.Errorf("Expected callers B and C, got %v", callerNames)
	}
}

func TestGetAllCallersWithStop_StopAtMultipleNodes(t *testing.T) {
	// Build a diamond graph:
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	// D is entry point, should stop there
	cg := NewCallGraph()

	a := &model.Symbol{Name: "A", Package: "pkg"}
	b := &model.Symbol{Name: "B", Package: "pkg"}
	c := &model.Symbol{Name: "C", Package: "pkg"}
	d := &model.Symbol{Name: "D", Package: "pkg"}

	cg.AddEdge(b, a) // B calls A
	cg.AddEdge(c, a) // C calls A
	cg.AddEdge(d, b) // D calls B
	cg.AddEdge(d, c) // D calls C

	// Stop at D - NEW BEHAVIOR: should get B, C, and D (D itself IS collected)
	stopFn := func(s *model.Symbol) bool {
		return s.Name == "D"
	}

	callers := cg.GetAllCallersWithStop(a, -1, stopFn)

	// Should get B, C, and D (D is the entry point and is collected)
	if len(callers) != 3 {
		t.Fatalf("Expected 3 callers (B, C, D), got %d", len(callers))
	}

	callerNames := make(map[string]bool)
	for _, caller := range callers {
		callerNames[caller.Name] = true
	}

	if !callerNames["B"] || !callerNames["C"] || !callerNames["D"] {
		t.Errorf("Expected callers B, C, and D, got %v", callerNames)
	}
}

func TestGetAllCallersWithStop_NeverWorseThanNoStop(t *testing.T) {
	// Build a complex call graph and verify that with-stop never returns more than no-stop
	cg := NewCallGraph()

	// Build: A <- B <- C <- D <- E
	//            ^--- F <-----|
	a := &model.Symbol{Name: "A", Package: "pkg"}
	b := &model.Symbol{Name: "B", Package: "pkg"}
	c := &model.Symbol{Name: "C", Package: "pkg"}
	d := &model.Symbol{Name: "D", Package: "pkg"}
	e := &model.Symbol{Name: "E", Package: "pkg"}
	f := &model.Symbol{Name: "F", Package: "pkg"}

	cg.AddEdge(b, a)
	cg.AddEdge(c, b)
	cg.AddEdge(d, c)
	cg.AddEdge(e, d)
	cg.AddEdge(f, b)
	cg.AddEdge(e, f)

	// Stop at D
	stopFn := func(s *model.Symbol) bool {
		return s.Name == "D"
	}

	callersNoStop := cg.GetAllCallers(a, -1)
	callersWithStop := cg.GetAllCallersWithStop(a, -1, stopFn)

	// With stop should have fewer or equal callers
	if len(callersWithStop) > len(callersNoStop) {
		t.Errorf("With-stop returned %d callers, no-stop returned %d. With-stop should never exceed no-stop!",
			len(callersWithStop), len(callersNoStop))
	}

	// Verify with-stop result ⊆ no-stop result
	noStopMap := make(map[string]bool)
	for _, caller := range callersNoStop {
		noStopMap[caller.QualifiedName()] = true
	}

	for _, caller := range callersWithStop {
		if !noStopMap[caller.QualifiedName()] {
			t.Errorf("With-stop returned caller %s which is not in no-stop result", caller.QualifiedName())
		}
	}
}

func TestGetAllCallersWithStop_InitialSymbolIsEntryPoint(t *testing.T) {
	// When the start symbol itself is a stop node, it gets collected (as it's visited)
	// This is consistent with the "collect stop nodes" semantic
	cg := NewCallGraph()

	a := &model.Symbol{Name: "A", Package: "pkg"}
	b := &model.Symbol{Name: "B", Package: "pkg"}

	cg.AddEdge(b, a) // B calls A

	// A itself is an entry point
	stopFn := func(s *model.Symbol) bool {
		return s.Name == "A"
	}

	callers := cg.GetAllCallersWithStop(a, -1, stopFn)

	// Should get A itself (start node is a stop node, so it's collected)
	// This is technically correct: if A is an entry point and we're analyzing A,
	// then A is an affected entry point
	if len(callers) != 1 {
		t.Fatalf("Expected 1 result (A itself as stop node), got %d", len(callers))
	}

	if callers[0].Name != "A" {
		t.Errorf("Expected A (start is stop node), got %s", callers[0].Name)
	}
}

func TestNoiseBareNameFallbackDoesNotCrossPackageInit(t *testing.T) {
	cg := NewCallGraph()
	aCaller := &model.Symbol{Name: "Caller", Package: "a", File: "a/a.go"}
	aInit := &model.Symbol{Name: "init", Package: "a", File: "a/a.go"}
	bInit := &model.Symbol{Name: "init", Package: "b", File: "b/b.go"}
	cg.AddEdge(aCaller, aInit)
	cg.Finalize()

	if callers := cg.GetCallers(bInit); len(callers) != 0 {
		t.Fatalf("GetCallers(b.init)=%v, want no callers from a.init", callers)
	}
}

func TestNoiseBareNameFallbackDoesNotCrossPackageClose(t *testing.T) {
	cg := NewCallGraph()
	aCaller := &model.Symbol{Name: "Use", Package: "a", File: "a/a.go"}
	aClose := &model.Symbol{Name: "Close", Package: "a", File: "a/a.go", Kind: model.SymbolKindMethod}
	bClose := &model.Symbol{Name: "Close", Package: "b", File: "b/b.go", Kind: model.SymbolKindMethod}
	cg.AddEdge(aCaller, aClose)
	cg.Finalize()

	if callers := cg.GetCallers(bClose); len(callers) != 0 {
		t.Fatalf("GetCallers(b.Close)=%v, want no callers from a.Close", callers)
	}
}

func TestGetCalleesResolvesBareName(t *testing.T) {
	cg := NewCallGraph()
	caller := &model.Symbol{Name: "Entry", Package: "pkg", File: "entry.go"}
	callee := &model.Symbol{Name: "Worker", Package: "pkg", File: "worker.go"}
	cg.AddSymbol(caller)
	cg.AddSymbol(callee)
	cg.AddEdge(caller, &model.Symbol{Name: "Worker"})
	cg.Finalize()
	callees := cg.GetCallees(caller)
	if len(callees) != 1 || callees[0].File != "worker.go" {
		t.Fatalf("expected resolved Worker callee, got %#v", callees)
	}
}

// Receiver owner hints recorded at AddEdge time narrow the bare-name
// fallback: a caller whose call site addressed a receiver inconsistent with
// the target's Owner is vetoed; a consistent hint keeps the caller even in
// strict (non-permissive) mode.
func TestBareNameFallbackReceiverHintNarrowsOwnerMatch(t *testing.T) {
	build := func(permissive bool) *CallGraph {
		cg := NewCallGraph()
		cg.SetPermissiveFallback(permissive)

		hintMatch := &model.Symbol{Name: "place", Owner: "OrderService", Package: "p", File: "p/order.go"}
		hintConflict := &model.Symbol{Name: "gen", Owner: "ReportService", Package: "p", File: "p/report.go"}
		noHint := &model.Symbol{Name: "legacy", Owner: "OtherService", Package: "p", File: "p/other.go"}
		sameFile := &model.Symbol{Name: "localHelper", Package: "p", File: "p/user_service.go"}
		multi := &model.Symbol{Name: "multi", Owner: "OtherService", Package: "p", File: "p/multi.go"}

		cg.AddEdge(hintMatch, &model.Symbol{Name: "find", Owner: "userService"})
		cg.AddEdge(hintConflict, &model.Symbol{Name: "find", Owner: "userRepo"})
		cg.AddEdge(noHint, &model.Symbol{Name: "find"})
		// Conflicting hint does not veto the same-file fallback.
		cg.AddEdge(sameFile, &model.Symbol{Name: "find", Owner: "userRepo"})
		// Same caller calling find() through two different receivers: the
		// conflicting hints merge to "" (ambiguous = no owner evidence).
		cg.AddEdge(multi, &model.Symbol{Name: "find", Owner: "userService"})
		cg.AddEdge(multi, &model.Symbol{Name: "find", Owner: "userRepo"})
		cg.Finalize()
		return cg
	}

	target := &model.Symbol{Name: "find", Owner: "UserService", Package: "p", File: "p/user_service.go"}

	names := func(callers []*model.Symbol) map[string]bool {
		out := make(map[string]bool, len(callers))
		for _, c := range callers {
			out[c.Name] = true
		}
		return out
	}

	// Permissive (default): consistent hint and no-hint callers are kept;
	// a contradicting hint vetoes the same-package fallback.
	got := names(build(true).GetCallers(target))
	for _, want := range []string{"place", "legacy", "localHelper", "multi"} {
		if !got[want] {
			t.Errorf("permissive: expected caller %q, got %v", want, got)
		}
	}
	if got["gen"] {
		t.Errorf("permissive: caller gen has contradicting hint userRepo, should be vetoed: %v", got)
	}

	// Strict: the consistent hint still vouches for its caller (cross-file,
	// same package); no-hint and ambiguous-hint callers are dropped.
	got = names(build(false).GetCallers(target))
	if !got["place"] {
		t.Errorf("strict: expected caller place kept via matching hint, got %v", got)
	}
	if !got["localHelper"] {
		t.Errorf("strict: expected same-file caller localHelper, got %v", got)
	}
	for _, unwanted := range []string{"gen", "legacy", "multi"} {
		if got[unwanted] {
			t.Errorf("strict: caller %q should be filtered out, got %v", unwanted, got)
		}
	}
}

// When the target has no Owner, receiver hints are not consulted at all.
func TestBareNameFallbackReceiverHintIgnoredWithoutTargetOwner(t *testing.T) {
	cg := NewCallGraph()
	caller := &model.Symbol{Name: "place", Owner: "OrderService", Package: "p", File: "p/order.go"}
	cg.AddEdge(caller, &model.Symbol{Name: "find", Owner: "userRepo"})
	cg.Finalize()

	target := &model.Symbol{Name: "find", Package: "p", File: "p/user_service.go"}
	callers := cg.GetCallers(target)
	if len(callers) != 1 || callers[0].Name != "place" {
		t.Fatalf("target without Owner should ignore hints, got %v", callers)
	}
}

// Forward direction (GetCallees): a callee placeholder carrying a receiver
// hint must not resolve to candidates whose declared owner contradicts the
// hint; when proximity rules match nothing, contradicted candidates are not
// used as fallback either.
func TestResolveCalleeReceiverHintNarrowsCandidates(t *testing.T) {
	cg := NewCallGraph()

	caller := &model.Symbol{Name: "place", Owner: "Checkout", Package: "svc", File: "svc/checkout.go"}
	userFind := &model.Symbol{Name: "find", Owner: "UserService", Package: "svc", File: "svc/user_service.go"}
	orderFind := &model.Symbol{Name: "find", Owner: "OrderService", Package: "svc", File: "svc/order_service.go"}
	cg.AddSymbol(userFind)
	cg.AddSymbol(orderFind)
	cg.AddEdge(caller, &model.Symbol{Name: "find", Owner: "orderService"})
	cg.Finalize()

	callees := cg.GetCallees(caller)
	if len(callees) != 1 || callees[0].Owner != "OrderService" {
		t.Fatalf("expected only OrderService.find, got %v", callees)
	}
}

// Without a receiver hint on the callee, forward resolution keeps all
// proximity-matching candidates (behavior unchanged).
func TestResolveCalleeWithoutHintKeepsCandidates(t *testing.T) {
	cg := NewCallGraph()

	caller := &model.Symbol{Name: "place", Owner: "Checkout", Package: "svc", File: "svc/checkout.go"}
	userFind := &model.Symbol{Name: "find", Owner: "UserService", Package: "svc", File: "svc/user_service.go"}
	orderFind := &model.Symbol{Name: "find", Owner: "OrderService", Package: "svc", File: "svc/order_service.go"}
	cg.AddSymbol(userFind)
	cg.AddSymbol(orderFind)
	cg.AddEdge(caller, &model.Symbol{Name: "find"})
	cg.Finalize()

	callees := cg.GetCallees(caller)
	if len(callees) != 2 {
		t.Fatalf("expected both find candidates without hint, got %v", callees)
	}
}

// Local-shadow rule: a package-less target's "qualified" index key is just
// its bare name, so bare call sites from every file land in the same bucket.
// A caller whose own file declares a same-named symbol (call-compatible kind)
// binds its bare call locally and must not be attributed to a target declared
// in another file; callers in files without a local declaration and same-file
// callers are kept.
func TestBareNameLocalShadowDropsCrossFileCaller(t *testing.T) {
	cg := NewCallGraph()
	tsSym := func(name, file string) *model.Symbol {
		return &model.Symbol{Name: name, Kind: model.SymbolKindFunction, File: file, Language: model.LanguageTypeScript}
	}
	libGreet := tsSym("greet", "src/lib.ts")
	cg.AddSymbol(libGreet)
	cg.AddSymbol(tsSym("greet", "src/other.ts")) // local shadow in other.ts
	serve := tsSym("serve", "src/svc.ts")
	useOwn := tsSym("useOwn", "src/other.ts")
	helper := tsSym("helper", "src/lib.ts")
	cg.AddSymbol(serve)
	cg.AddSymbol(useOwn)
	cg.AddSymbol(helper)

	// Call-site callees as the TS adapter emits them: bare name, no package.
	cg.AddEdge(serve, &model.Symbol{Name: "greet", Language: model.LanguageTypeScript})
	cg.AddEdge(useOwn, &model.Symbol{Name: "greet", Language: model.LanguageTypeScript})
	cg.AddEdge(helper, &model.Symbol{Name: "greet", Language: model.LanguageTypeScript})
	cg.Finalize()

	names := func(callers []*model.Symbol) map[string]bool {
		out := make(map[string]bool, len(callers))
		for _, c := range callers {
			out[c.Name] = true
		}
		return out
	}

	got := names(cg.GetCallers(libGreet))
	if !got["serve"] || !got["helper"] {
		t.Errorf("expected callers serve (importing file) and helper (same file), got %v", got)
	}
	if got["useOwn"] {
		t.Errorf("useOwn's bare call binds to other.ts's own greet; must not be attributed to lib.ts greet: %v", got)
	}

	got = names(cg.GetCallersByKind(libGreet, model.EdgeKindCall))
	if !got["serve"] || !got["helper"] || got["useOwn"] {
		t.Errorf("GetCallersByKind(call) inconsistent with GetCallers: %v", got)
	}
}

// The bare-name fallback applies the same local-shadow rule, but only to
// genuinely bare calls: an entry carrying a receiver hint is a member call,
// which a top-level local declaration does not bind.
func TestBareNameFallbackLocalShadowRespectsReceiverHint(t *testing.T) {
	cg := NewCallGraph()
	target := &model.Symbol{Name: "greet", Kind: model.SymbolKindFunction, File: "src/lib.ts"}
	cg.AddSymbol(target)
	cg.AddSymbol(&model.Symbol{Name: "greet", Kind: model.SymbolKindFunction, File: "src/other.ts"})
	bare := &model.Symbol{Name: "useBare", Kind: model.SymbolKindFunction, File: "src/other.ts"}
	member := &model.Symbol{Name: "useMember", Kind: model.SymbolKindFunction, File: "src/other.ts"}

	// Package/owner hints keep these callees out of the target's
	// qualified-name bucket, forcing the bare-name fallback.
	cg.AddEdge(bare, &model.Symbol{Name: "greet", Package: "lib"})
	cg.AddEdge(member, &model.Symbol{Name: "greet", Owner: "greeter"})
	cg.Finalize()

	got := make(map[string]bool)
	for _, c := range cg.GetCallers(target) {
		got[c.Name] = true
	}
	if got["useBare"] {
		t.Errorf("bare call in a file declaring its own greet binds locally, should be dropped: %v", got)
	}
	if !got["useMember"] {
		t.Errorf("member call greeter.greet() is not bound by the local top-level greet, should be kept: %v", got)
	}
}

// Kind compatibility: a variable/constant holding a function value shadows a
// function target; an unrelated kind (interface) does not.
func TestBareNameLocalShadowKindCompatibility(t *testing.T) {
	build := func(localKind model.SymbolKind) *CallGraph {
		cg := NewCallGraph()
		cg.AddSymbol(&model.Symbol{Name: "greet", Kind: model.SymbolKindFunction, File: "src/lib.ts"})
		cg.AddSymbol(&model.Symbol{Name: "greet", Kind: localKind, File: "src/other.ts"})
		caller := &model.Symbol{Name: "useOwn", Kind: model.SymbolKindFunction, File: "src/other.ts"}
		cg.AddEdge(caller, &model.Symbol{Name: "greet"})
		cg.Finalize()
		return cg
	}
	target := &model.Symbol{Name: "greet", Kind: model.SymbolKindFunction, File: "src/lib.ts"}

	if callers := build(model.SymbolKindVariable).GetCallers(target); len(callers) != 0 {
		t.Errorf("local variable holding a function value should shadow, got %v", callers)
	}
	if callers := build(model.SymbolKindInterface).GetCallers(target); len(callers) != 1 {
		t.Errorf("local interface cannot bind a call, should not shadow, got %v", callers)
	}
}

// The rule never applies to targets with a package: their qualified-name
// bucket contains only genuinely qualified matches, which stay authoritative.
func TestLocalShadowSkippedForPackagedTarget(t *testing.T) {
	cg := NewCallGraph()
	target := &model.Symbol{Name: "greet", Package: "lib", Kind: model.SymbolKindFunction, File: "lib/lib.go"}
	caller := &model.Symbol{Name: "useOwn", Package: "other", Kind: model.SymbolKindFunction, File: "other/other.go"}
	cg.AddSymbol(&model.Symbol{Name: "greet", Package: "other", Kind: model.SymbolKindFunction, File: "other/other.go"})
	cg.AddEdge(caller, &model.Symbol{Name: "greet", Package: "lib"})
	cg.Finalize()

	callers := cg.GetCallers(target)
	if len(callers) != 1 || callers[0].Name != "useOwn" {
		t.Fatalf("packaged target must ignore the local-shadow rule, got %v", callers)
	}
}

// A target without file info cannot be distinguished from the local
// declaration itself, so the rule stays off.
func TestLocalShadowSkippedWithoutTargetFile(t *testing.T) {
	cg := NewCallGraph()
	caller := &model.Symbol{Name: "useOwn", Kind: model.SymbolKindFunction, File: "src/other.ts"}
	cg.AddSymbol(&model.Symbol{Name: "greet", Kind: model.SymbolKindFunction, File: "src/other.ts"})
	cg.AddEdge(caller, &model.Symbol{Name: "greet"})
	cg.Finalize()

	target := &model.Symbol{Name: "greet", Kind: model.SymbolKindFunction}
	callers := cg.GetCallers(target)
	if len(callers) != 1 || callers[0].Name != "useOwn" {
		t.Fatalf("file-less target must not trigger the local-shadow rule, got %v", callers)
	}
}
