package orchestrator

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

// TestInterfaceImplsPythonBaseClass verifies that a change to a Python base
// class populates InterfaceImpls from the adapter's inheritance edges
// (subclass -> base class), mirroring the Java interface path.
func TestInterfaceImplsPythonBaseClass(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "base.py", `class Animal:
    def speak(self):
        return "..."
`)
	repo.WriteFile(t, "dog.py", `from base import Animal


class Dog(Animal):
    def speak(self):
        return "woof"
`)
	repo.Commit(t, "initial")
	repo.WriteFile(t, "base.py", `class Animal:
    def speak(self, loud=False):
        return "..."
`)
	repo.Commit(t, "change")

	result := analyzeMiniRepo(t, repo)
	impls := interfaceImplsOf(result, "Animal")
	if !hasSymbolNamed(impls, "Dog") {
		t.Fatalf("InterfaceImpls for Animal = %v, want subclass Dog", symbolNames(impls))
	}
}

// TestInterfaceImplsRustTrait verifies that a change to a Rust trait populates
// InterfaceImpls from the adapter's inheritance edges (impl Trait for Type),
// whose caller comes from the adapter-attached CallSite.Caller because impl
// blocks are not symbols.
func TestInterfaceImplsRustTrait(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "Cargo.toml", "[package]\nname = \"mini\"\nversion = \"0.1.0\"\n")
	repo.WriteFile(t, "src/lib.rs", `pub trait Greeter {
    fn greet(&self) -> String;
}

pub struct English;

impl Greeter for English {
    fn greet(&self) -> String {
        "hi".to_string()
    }
}
`)
	repo.Commit(t, "initial")
	repo.WriteFile(t, "src/lib.rs", `pub trait Greeter {
    fn greet(&self, punct: &str) -> String;
}

pub struct English;

impl Greeter for English {
    fn greet(&self, punct: &str) -> String {
        format!("hi{}", punct)
    }
}
`)
	repo.Commit(t, "change")

	result := analyzeMiniRepo(t, repo)
	impls := interfaceImplsOf(result, "Greeter")
	if !hasSymbolNamed(impls, "English") {
		t.Fatalf("InterfaceImpls for Greeter = %v, want implementor English", symbolNames(impls))
	}
}

func analyzeMiniRepo(t *testing.T, repo *testutil.MiniRepo) *model.ImpactResult {
	t.Helper()
	o, err := NewOrchestrator(repo.Dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	result, err := o.AnalyzeDiffWithSurface("HEAD~1", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// interfaceImplsOf returns the InterfaceImpls of the change whose symbol (or
// enclosing interface-like symbol) has the given name.
func interfaceImplsOf(result *model.ImpactResult, name string) []*model.Symbol {
	for _, ch := range result.Changes {
		if ch.Change == nil || ch.Change.Symbol == nil {
			continue
		}
		sym := ch.Change.Symbol
		if sym.Name == name || sym.Owner == name {
			return ch.InterfaceImpls
		}
	}
	return nil
}

func hasSymbolNamed(syms []*model.Symbol, name string) bool {
	for _, sym := range syms {
		if sym != nil && sym.Name == name {
			return true
		}
	}
	return false
}

func symbolNames(syms []*model.Symbol) []string {
	var names []string
	for _, sym := range syms {
		if sym != nil {
			names = append(names, sym.Name)
		}
	}
	return names
}
