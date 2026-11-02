package diff

import (
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

func TestGitDiffAndRefsWithMiniRepo(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 1 }\n")
	first := repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 2 }\n")
	second := repo.Commit(t, "modify")

	g := NewGit(repo.Dir)
	if head, err := g.GetHEAD(); err != nil || head != second {
		t.Fatalf("GetHEAD=(%q,%v), want %q", head, err, second)
	}
	if resolved, err := g.ResolveRef("HEAD~1"); err != nil || resolved != first {
		t.Fatalf("ResolveRef=(%q,%v), want %q", resolved, err, first)
	}
	out, err := g.Diff("HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "return 2") {
		t.Fatalf("diff output missing modified content:\n%s", out)
	}
	files, err := g.ChangedFiles("HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "a.go" {
		t.Fatalf("changed files=%v, want [a.go]", files)
	}
}

func TestGitDiffWithChangedFiles(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 1 }\n")
	repo.WriteFile(t, "dir/file with space.go", "package dir\nfunc B() int { return 1 }\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 2 }\n")
	repo.WriteFile(t, "dir/file with space.go", "package dir\nfunc B() int { return 2 }\n")
	repo.Commit(t, "modify")

	g := NewGit(repo.Dir)
	diffText, files, err := g.DiffWithChangedFiles("HEAD~1")
	if err != nil {
		t.Fatal(err)
	}

	wantDiff, err := g.Diff("HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if diffText != wantDiff {
		t.Fatal("DiffWithChangedFiles diff text differs from Diff")
	}

	wantFiles, err := g.ChangedFiles("HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(files, ",") != strings.Join(wantFiles, ",") {
		t.Fatalf("DiffWithChangedFiles files=%v, want %v", files, wantFiles)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 changed files, got %v", files)
	}
}
