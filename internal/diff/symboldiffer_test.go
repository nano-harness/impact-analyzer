package diff

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

func diffFromRepo(t *testing.T, repo *testutil.MiniRepo, ref string) []*FileDiff {
	t.Helper()
	text, err := NewGit(repo.Dir).Diff(ref)
	if err != nil {
		t.Fatal(err)
	}
	fds, err := ParseUnifiedDiff(text)
	if err != nil {
		t.Fatal(err)
	}
	return fds
}

func changesFromRepo(t *testing.T, repo *testutil.MiniRepo, ref string, cg *graph.CallGraph) []*model.Change {
	t.Helper()
	changes, err := NewSymbolDiffer(repo.Dir, ref, cg).Diff(diffFromRepo(t, repo, ref))
	if err != nil {
		t.Fatal(err)
	}
	return changes
}

func effectiveChanges(changes []*model.Change) []*model.Change {
	var out []*model.Change
	for _, change := range changes {
		if !change.NoiseFiltered {
			out = append(out, change)
		}
	}
	return out
}

func TestSymbolDifferModifiedFunctionBody(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() int {\n\treturn 2\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int {\n\treturn 10\n}\n\nfunc B() int {\n\treturn 2\n}\n")
	repo.Commit(t, "modify")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Type != model.ChangeTypeModified || changes[0].Symbol == nil || changes[0].Symbol.Name != "A" {
		t.Fatalf("changes=%+v, want one modified change for A", changes)
	}
	if !changes[0].BodyChanged {
		t.Fatal("body change should set BodyChanged")
	}
}

func TestSymbolDifferDeletedFunctionUsesOldSymbol(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc Gone() int {\n\treturn 1\n}\n\nfunc Next() int {\n\treturn 2\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nfunc Next() int {\n\treturn 2\n}\n")
	repo.Commit(t, "delete")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Type != model.ChangeTypeDeleted || changes[0].Symbol == nil || changes[0].Symbol.Name != "Gone" {
		t.Fatalf("changes=%+v, want one deleted change for old symbol Gone", changes)
	}
}

func TestSymbolDifferAddedFunction(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int { return 1 }\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int { return 1 }\n\nfunc Added() int { return 2 }\n")
	repo.Commit(t, "add")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Type != model.ChangeTypeAdded || changes[0].Symbol == nil || changes[0].Symbol.Name != "Added" {
		t.Fatalf("changes=%+v, want one added change for Added", changes)
	}
}

func TestSymbolDifferPureRename(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int { return 1 }\n")
	repo.Commit(t, "initial")
	repo.Rename(t, "a.go", "b.go")
	repo.Commit(t, "rename")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Type != model.ChangeTypeRenamed || changes[0].Symbol == nil || changes[0].OldSymbol == nil || changes[0].Symbol.Name != "A" || changes[0].File != "b.go" {
		t.Fatalf("changes=%+v, want one renamed change with old and new symbol A", changes)
	}
}

func TestSymbolDifferNoiseOnlyChanges(t *testing.T) {
	cases := []struct {
		name   string
		before string
		after  string
	}{
		{"comment", "package p\n\nfunc A() int {\n\treturn 1\n}\n", "package p\n\n// comment\nfunc A() int {\n\treturn 1\n}\n"},
		{"blank", "package p\n\nfunc A() int {\n\treturn 1\n}\n", "package p\n\n\nfunc A() int {\n\treturn 1\n}\n"},
		{"whitespace", "package p\n\nfunc A() int {\n\treturn 1\n}\n", "package p\n\nfunc A() int {\n    return 1\n}\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := testutil.NewMiniRepo(t)
			repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
			repo.WriteFile(t, "a.go", tc.before)
			repo.Commit(t, "initial")
			repo.WriteFile(t, "a.go", tc.after)
			repo.Commit(t, "noise")

			changes := changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph())
			if len(effectiveChanges(changes)) != 0 {
				t.Fatalf("effective changes=%+v, want none", effectiveChanges(changes))
			}
			if len(changes) == 0 || !changes[0].NoiseFiltered {
				t.Fatalf("changes=%+v, want noise marker", changes)
			}
		})
	}
}

func TestSymbolDifferMixedCommentAndCodeFiltersOnlyComment(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int {\n\t// old\n\treturn 1\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int {\n\t// new\n\treturn 2\n}\n")
	repo.Commit(t, "mixed")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Symbol == nil || changes[0].Symbol.Name != "A" || changes[0].CommentOnly {
		t.Fatalf("changes=%+v, want one non-comment-only change for A", changes)
	}
}

func TestSymbolDifferStructFieldFallback(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\ntype User struct {\n\tName string\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\ntype User struct {\n\tName string\n\tAge int\n}\n")
	repo.Commit(t, "field")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Symbol == nil || changes[0].Symbol.Name != "User" || !changes[0].OutOfFunction || changes[0].FallbackKind != "struct_field" {
		t.Fatalf("changes=%+v, want struct field fallback to User", changes)
	}
}

func TestSymbolDifferTopLevelVarFallbackToCallers(t *testing.T) {
	cg := graph.NewCallGraph()
	varSym := &model.Symbol{Name: "Config", Kind: model.SymbolKindVariable, Package: "p", File: "a.go", Line: 3, EndLine: 3, Language: model.LanguageGo}
	caller1 := &model.Symbol{Name: "UseOne", Kind: model.SymbolKindFunction, Package: "p", File: "a.go", Line: 5, EndLine: 7, Language: model.LanguageGo}
	caller2 := &model.Symbol{Name: "UseTwo", Kind: model.SymbolKindFunction, Package: "p", File: "a.go", Line: 9, EndLine: 11, Language: model.LanguageGo}
	cg.AddEdge(caller1, varSym)
	cg.AddEdge(caller2, varSym)
	cg.Finalize()

	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nvar Config = 1\n\nfunc UseOne() int { return Config }\nfunc UseTwo() int { return Config }\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nvar Config = 2\n\nfunc UseOne() int { return Config }\nfunc UseTwo() int { return Config }\n")
	repo.Commit(t, "var")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", cg))
	if len(changes) != 2 {
		t.Fatalf("changes=%+v, want two caller changes", changes)
	}
	seen := map[string]bool{}
	for _, change := range changes {
		if !change.OutOfFunction || change.FallbackKind != "var_ref" || change.Symbol == nil {
			t.Fatalf("change=%+v, want var_ref caller", change)
		}
		seen[change.Symbol.Name] = true
	}
	if !seen["UseOne"] || !seen["UseTwo"] {
		t.Fatalf("seen=%v, want UseOne and UseTwo", seen)
	}
}

func TestSymbolDifferImportFallbackDoesNotEnterBFS(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nimport \"fmt\"\n\nfunc A() { fmt.Println(1) }\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc A() { fmt.Println(strings.TrimSpace(\" x \")) }\n")
	repo.Commit(t, "import")

	changes := changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph())
	var fileLevel bool
	for _, change := range changes {
		if change.OutOfFunction && change.FallbackKind == "file_level" && change.Symbol == nil {
			fileLevel = true
		}
	}
	if !fileLevel {
		t.Fatalf("changes=%+v, want file_level fallback marker", changes)
	}
}

func TestSymbolDifferSignatureChanged(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A(v int) int { return v }\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A(v int, w int) int { return v + w }\n")
	repo.Commit(t, "signature")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Symbol == nil || changes[0].Symbol.Name != "A" || !changes[0].SignatureChanged {
		t.Fatalf("changes=%+v, want signature change for A", changes)
	}
}

func TestSymbolDifferMergesMultipleHunksForSameFunction(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int {\n\tx := 1\n\ty := 2\n\tz := 3\n\treturn x + y + z\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "a.go", "package p\n\nfunc A() int {\n\tx := 10\n\ty := 2\n\tz := 30\n\treturn x + y + z\n}\n")
	repo.Commit(t, "two hunks")

	changes := effectiveChanges(changesFromRepo(t, repo, "HEAD~1", graph.NewCallGraph()))
	if len(changes) != 1 || changes[0].Symbol == nil || changes[0].Symbol.Name != "A" {
		t.Fatalf("changes=%+v, want merged change for A", changes)
	}
}
