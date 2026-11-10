package godeep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzer_BuildSSA_RecoversProgramCreationPanic(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.com/creationpanic\n\ngo 1.25\n",
		"main.go": "package main\nfunc main() { println(1 + 2) }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := NewAnalyzer()
	if err := a.LoadPackages(dir); err != nil {
		t.Fatal(err)
	}
	// Incomplete definition metadata deterministically panics during program
	// creation, before x/tools starts parallel package-builder workers.
	a.pkgs[0].TypesInfo.Defs = nil
	err := a.BuildSSA()
	if err == nil || !strings.Contains(err.Error(), "SSA construction panicked") {
		t.Fatalf("expected a recovered SSA construction error, got %v", err)
	}
	if a.Prog != nil || a.mains != nil {
		t.Fatal("failed creation must discard partial SSA state")
	}
	if err := a.LoadPackages(dir); err != nil {
		t.Fatal(err)
	}
	if err := a.BuildSSA(); err != nil {
		t.Fatalf("fresh build after panic failed: %v", err)
	}
	if a.Prog == nil {
		t.Fatal("fresh build did not produce an SSA program")
	}
}

// Cover the historical RTA route (empty custom entry points) with generic
// types flowing through interfaces, not only a directly called generic Sum.
func TestGenericInterfaces_StandaloneAndSharedSession(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod": "module example.com/genericinterfaces\n\ngo 1.25\n",
		"main.go": `package fixture

type Reader[T any] interface { Read() T }
type Box[T any] struct { Value T }
type Alias[T any] = Box[T]
func identity[T any](value T) T { return value }
func (box Box[T]) Read() T { return identity(box.Value) }
func emit(value any) {}
func Entry() {
	var reader Reader[int] = Alias[int]{Value: 42}
	emit(reader.Read())
}
func init() { Entry() }
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a := NewAnalyzer()
	if err := a.LoadPackages(dir); err != nil {
		t.Fatal(err)
	}
	if err := a.BuildSSA(); err != nil {
		t.Fatal(err)
	}
	graph, err := a.BuildCallGraph()
	if err != nil {
		t.Fatalf("standalone generic RTA failed: %v", err)
	}
	reachable := make(map[string]bool)
	for fn := range graph.Nodes {
		if fn != nil {
			reachable[strings.SplitN(fn.Name(), "[", 2)[0]] = true
		}
	}
	for _, name := range []string{"Read", "identity", "emit"} {
		if !reachable[name] {
			t.Errorf("standalone RTA lost %s through the generic interface", name)
		}
	}

	positions := make(map[string]bool)
	for pos := range a.ReachableFunctionPositions(graph) {
		positions[pos.Name] = true
	}
	symbols := make(map[string]bool)
	for _, sym := range a.ConvertToModel(graph) {
		if sym.Package == "example.com/genericinterfaces" {
			symbols[sym.Name] = true
		}
	}
	for _, name := range []string{"Read", "identity", "emit"} {
		if !positions[name] || !symbols[name] {
			t.Errorf("missing source identity for %s: position=%v symbol=%v", name, positions[name], symbols[name])
		}
	}

	session, err := LoadSession(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := session.CallSitesForDir(dir)
	if err != nil {
		t.Fatalf("shared-session generic RTA failed: %v", err)
	}
	callees := make(map[string]bool)
	for _, call := range calls {
		if call.Callee != nil {
			callees[call.Callee.Name] = true
		}
	}
	for _, name := range []string{"Read", "identity", "emit"} {
		if !callees[name] {
			t.Errorf("shared-session RTA lost %s through the generic interface", name)
		}
	}
}
