package diff

import (
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/golang"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

func TestOldFileLoader_RevisionRanges(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 0 }\n")
	base := repo.Commit(t, "base")
	repo.Git(t, "checkout", "-b", "left")
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 1 }\n")
	repo.Commit(t, "left")
	repo.Git(t, "checkout", "-b", "right", base)
	repo.WriteFile(t, "a.go", "package p\nfunc A() int { return 2 }\n")
	repo.Commit(t, "right")

	for _, tc := range []struct{ ref, want string }{
		{"left", "return 1"},
		{"left..right", "return 1"},
		{"left...right", "return 0"},
		{"left...", "return 0"},
		{"...left", "return 0"},
		{"..left", "return 2"},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			got, err := NewOldFileLoader(repo.Dir, tc.ref).LoadOldContent("a.go")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("old side of %q = %q, want %s", tc.ref, got, tc.want)
			}
		})
	}
}

func TestSymbolDiffer_TripleDotGenericBodyChange(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	before := "package p\ntype Box[T any] struct { Value T }\nfunc (b Box[T]) Read() T { return b.Value }\n"
	repo.WriteFile(t, "a.go", before)
	repo.Commit(t, "base")
	repo.Git(t, "branch", "base")
	repo.WriteFile(t, "a.go", strings.Replace(before, "return b.Value", "value := b.Value; return value", 1))
	repo.Commit(t, "body change")

	text, err := NewGit(repo.Dir).Diff("base...HEAD")
	if err != nil {
		t.Fatal(err)
	}
	files, err := ParseUnifiedDiff(text)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := NewSymbolDiffer(repo.Dir, "base...HEAD", graph.NewCallGraph()).Diff(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Type != model.ChangeTypeModified || changes[0].SignatureChanged || !changes[0].BodyChanged {
		t.Fatalf("body-only change must not split into added/deleted or change signature: %+v", changes)
	}
}
