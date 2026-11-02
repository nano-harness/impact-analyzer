package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
	"github.com/spf13/cobra"
)

func TestAnalyzeCommandCommentOnlyJSON(t *testing.T) {
	repo := testutil.NewMiniRepo(t)
	repo.WriteFile(t, "go.mod", "module example.com/mini\n\ngo 1.25\n")
	repo.WriteFile(t, "main.go", "package main\n\nfunc main() {\n\tService()\n}\n\nfunc Service() int {\n\treturn 1\n}\n")
	repo.Commit(t, "initial")
	repo.WriteFile(t, "main.go", "package main\n\n// package comment\nfunc main() {\n\tService()\n}\n\nfunc Service() int {\n\treturn 1\n}\n")
	repo.Commit(t, "comment")

	restore := chdir(t, repo.Dir)
	defer restore()

	out := captureStdout(t, func() {
		analyzeDiff = "HEAD~1"
		analyzeFile = ""
		analyzeDepth = 3
		analyzeExclude = nil
		analyzeNoSurface = true
		format = "json"
		if err := runAnalyze(&cobra.Command{}, nil); err != nil {
			t.Fatal(err)
		}
	})
	var result model.ImpactResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("invalid JSON output %q: %v", out, err)
	}
	if result.TotalReach != 0 {
		t.Fatalf("comment-only total reach=%d, want 0", result.TotalReach)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
