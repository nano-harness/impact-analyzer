package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/lang/golang/godeep"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// TestSharedSessionMatchesStandalone verifies that the shared-session deep
// analysis (one LoadPackages over root+"/...", per-directory RTA) produces
// the same call sites as the legacy standalone per-directory
// BuildPackageCallGraph for the packages under this directory.
func TestSharedSessionMatchesStandalone(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	session, err := godeep.LoadSession(cwd, nil)
	if err != nil {
		t.Skipf("skipping - requires Go module context: %v", err)
	}

	adapter := &GoAdapter{}
	dirs := []string{cwd, filepath.Join(cwd, "godeep")}

	keyOf := func(c *model.CallSite) string {
		return fmt.Sprintf("%s:%d:%d|%s@%s:%d",
			c.File, c.Line, c.Column,
			c.Callee.QualifiedName(), c.Callee.File, c.Callee.Line)
	}
	setOf := func(calls []*model.CallSite) map[string]bool {
		m := make(map[string]bool, len(calls))
		for _, c := range calls {
			if c == nil || c.Callee == nil {
				continue
			}
			m[keyOf(c)] = true
		}
		return m
	}

	for _, dir := range dirs {
		if !session.CoversDir(dir) {
			t.Errorf("session does not cover %s", dir)
			continue
		}

		legacy, err := adapter.BuildPackageCallGraph(dir, nil, nil)
		if err != nil {
			t.Fatalf("standalone BuildPackageCallGraph failed for %s: %v", dir, err)
		}
		shared, err := session.CallSitesForDir(dir)
		if err != nil {
			t.Fatalf("session CallSitesForDir failed for %s: %v", dir, err)
		}

		lset, sset := setOf(legacy), setOf(shared)

		var missing []string
		for k := range lset {
			if !sset[k] {
				missing = append(missing, k)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("dir %s: %d call sites lost by shared session:\n%v", dir, len(missing), missing)
		}

		extra := 0
		for k := range sset {
			if !lset[k] {
				extra++
			}
		}
		if extra > 0 {
			t.Logf("dir %s: %d additional call sites from shared session (acceptable)", dir, extra)
		}
	}
}

// TestSharedSessionUncoveredDir ensures CallSitesForDir reports an error for
// directories outside the session root so callers can fall back to the
// legacy per-directory path.
func TestSharedSessionUncoveredDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	session, err := godeep.LoadSession(cwd, nil)
	if err != nil {
		t.Skipf("skipping - requires Go module context: %v", err)
	}

	if session.CoversDir(os.TempDir()) {
		t.Fatalf("session unexpectedly covers %s", os.TempDir())
	}
	if _, err := session.CallSitesForDir(os.TempDir()); err == nil {
		t.Fatal("expected error for directory outside the session root")
	}
}

func TestStandaloneGenericCallSitesUseSourceNames(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.com/genericcalls\n\ngo 1.25\n",
		"main.go": "package main\nfunc identity[T any](value T) T { return value }\nfunc main() { _ = identity(1) }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	calls, err := (&GoAdapter{}).BuildPackageCallGraph(dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call.Callee != nil && call.Callee.Name == "identity" && call.Callee.Package == "example.com/genericcalls" {
			return
		}
	}
	t.Fatal("standalone conversion dropped the instantiated identity call")
}
