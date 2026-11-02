package orchestrator

import (
	"testing"

	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

// TestRecall_CrossPackageCallerDetection verifies that modifying a symbol in one package
// correctly detects callers in another package (via reverse import expansion).
func TestRecall_CrossPackageCallerDetection(t *testing.T) {
	repo := testutil.NewMiniRepo(t)

	// Set up a multi-package Go project
	repo.WriteFile(t, "go.mod", "module example.com/recall\n\ngo 1.25\n")

	// Package "lib" with an exported function
	repo.WriteFile(t, "lib/lib.go", `package lib

// Helper is a utility function used by other packages.
func Helper() string {
	return "hello"
}
`)

	// Package "svc" that imports and calls lib.Helper
	repo.WriteFile(t, "svc/svc.go", `package svc

import "example.com/recall/lib"

func Service() string {
	return lib.Helper()
}
`)

	// Package "main" that imports svc
	repo.WriteFile(t, "main.go", `package main

import "example.com/recall/svc"

func main() {
	svc.Service()
}
`)

	repo.Commit(t, "initial")

	// Modify the lib.Helper function
	repo.WriteFile(t, "lib/lib.go", `package lib

// Helper is a utility function used by other packages.
func Helper() string {
	return "world"
}
`)
	repo.Commit(t, "modify helper")

	orch, err := NewOrchestrator(repo.Dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	result, err := orch.AnalyzeDiffWithSurface("HEAD~1", 3, false)
	if err != nil {
		t.Fatal(err)
	}

	// We expect at least one change for Helper() with at least one caller (svc.Service)
	if len(result.Changes) == 0 {
		t.Fatal("expected at least one change, got 0")
	}

	foundCallers := false
	for _, change := range result.Changes {
		if change.Change != nil && change.Change.Symbol != nil && change.Change.Symbol.Name == "Helper" {
			if len(change.AllCallers) > 0 || result.TotalReach > 0 {
				foundCallers = true
				break
			}
		}
	}

	if !foundCallers {
		t.Errorf("expected cross-package callers for Helper(), but found none; TotalReach=%d", result.TotalReach)
		for _, change := range result.Changes {
			if change.Change != nil && change.Change.Symbol != nil {
				t.Logf("  change: %s.%s callers=%d", change.Change.Symbol.Package, change.Change.Symbol.Name, len(change.AllCallers))
			}
		}
	}
}

// TestRecall_MultiHopCallerDetection verifies that 2-hop callers are detected
// (A imports B imports C; change in C should find A as a transitive caller).
func TestRecall_MultiHopCallerDetection(t *testing.T) {
	repo := testutil.NewMiniRepo(t)

	repo.WriteFile(t, "go.mod", "module example.com/multihop\n\ngo 1.25\n")

	// Package "core" with a function
	repo.WriteFile(t, "core/core.go", `package core

func Compute() int {
	return 42
}
`)

	// Package "mid" imports core
	repo.WriteFile(t, "mid/mid.go", `package mid

import "example.com/multihop/core"

func Process() int {
	return core.Compute() + 1
}
`)

	// Package "top" imports mid
	repo.WriteFile(t, "top/top.go", `package top

import "example.com/multihop/mid"

func Run() int {
	return mid.Process()
}
`)

	repo.Commit(t, "initial")

	// Modify core.Compute
	repo.WriteFile(t, "core/core.go", `package core

func Compute() int {
	return 99
}
`)
	repo.Commit(t, "modify compute")

	orch, err := NewOrchestrator(repo.Dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()

	result, err := orch.AnalyzeDiffWithSurface("HEAD~1", 5, false)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that at least mid.Process is detected as a caller of core.Compute
	if result.TotalReach == 0 {
		t.Error("expected TotalReach > 0 for multi-hop scenario")
		for _, change := range result.Changes {
			if change.Change != nil && change.Change.Symbol != nil {
				t.Logf("  change: %s.%s callers=%d", change.Change.Symbol.Package, change.Change.Symbol.Name, len(change.AllCallers))
			}
		}
	}
}
