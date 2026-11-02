package orchestrator

import (
	"testing"

	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
)

func TestOrchestratorAnalyzeDiffCommentOnlyHasNoReach(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "main.go", "package main\n\nfunc main() {\n\tService()\n}\n\nfunc Service() int {\n\treturn 1\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "main.go", "package main\n\n// package-level comment only\nfunc main() {\n\tService()\n}\n\nfunc Service() int {\n\treturn 1\n}\n")
	repo.Commit(t, "comment")

	orch, err := NewOrchestrator(repo.Dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer orch.Close()
	result, err := orch.AnalyzeDiffWithSurface("HEAD~1", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalReach != 0 {
		t.Fatalf("comment-only reach=%d, want 0", result.TotalReach)
	}
}
