package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPythonResolverResolvesModuleAndPackage(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.Mkdir(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "__init__.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "mod.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	r := NewPythonResolver(root)
	got, err := r.ResolveImport("pkg.mod", filepath.Join(root, "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "mod.py" {
		t.Fatalf("got %s", got)
	}
	got, err = r.ResolveImport("pkg", filepath.Join(root, "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "__init__.py" {
		t.Fatalf("got %s", got)
	}
}
