package cobra

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPackageSSA_GoWorkMultiModule(t *testing.T) {
	ResetSSACache()
	root := t.TempDir()
	writeGoWorkspaceFixture(t, root)

	pkgDir := filepath.Join(root, "mainmod", "cmd", "app")
	prog, loadedDir, err := LoadPackageSSA(pkgDir)
	if err != nil {
		t.Fatalf("LoadPackageSSA failed: %v", err)
	}
	if prog == nil {
		t.Fatal("LoadPackageSSA returned nil program")
	}
	if loadedDir != pkgDir {
		t.Fatalf("loadedDir = %q, want %q", loadedDir, pkgDir)
	}
}

func TestLoadPackageSSA_CacheKeyIsPkgDir(t *testing.T) {
	ResetSSACache()
	root := t.TempDir()
	writeGoWorkspaceFixture(t, root)

	pkgDir := filepath.Join(root, "mainmod", "cmd", "app")
	prog1, loadedDir1, err := LoadPackageSSA(pkgDir)
	if err != nil {
		t.Fatalf("first LoadPackageSSA failed: %v", err)
	}
	prog2, loadedDir2, err := LoadPackageSSA(filepath.Join(pkgDir, "main.go"))
	if err != nil {
		t.Fatalf("second LoadPackageSSA failed: %v", err)
	}
	if prog1 != prog2 {
		t.Fatal("expected same *ssa.Program for repeated loads of same package directory")
	}
	if loadedDir1 != pkgDir || loadedDir2 != pkgDir {
		t.Fatalf("loaded dirs = %q, %q; want %q", loadedDir1, loadedDir2, pkgDir)
	}

	globalSSACache.mu.RLock()
	_, ok := globalSSACache.entries[pkgDir]
	cacheSize := len(globalSSACache.entries)
	globalSSACache.mu.RUnlock()
	if !ok {
		t.Fatalf("cache missing package dir key %q", pkgDir)
	}
	if cacheSize != 1 {
		t.Fatalf("cache size = %d, want 1", cacheSize)
	}
}

func writeGoWorkspaceFixture(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "go.work"), `go 1.22

use (
	./mainmod
	./tools/cli-eval
)
`)
	writeFile(t, filepath.Join(root, "mainmod", "go.mod"), "module example.com/mainmod\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "mainmod", "cmd", "app", "main.go"), `package app

func NewCmd() string {
	return "ok"
}
`)
	writeFile(t, filepath.Join(root, "tools", "cli-eval", "go.mod"), "module example.com/cli-eval\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "tools", "cli-eval", "broken.go"), `package broken

import "example.com/does-not-exist"
`)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
