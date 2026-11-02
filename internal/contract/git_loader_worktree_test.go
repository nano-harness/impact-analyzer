package contract

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

type worktreeTestExtractor struct{}

func (worktreeTestExtractor) Name() string                    { return "worktree-test" }
func (worktreeTestExtractor) Kind() model.ContractKind        { return model.ContractKindCLICommand }
func (worktreeTestExtractor) Language() model.Language        { return model.LanguageGo }
func (worktreeTestExtractor) CanHandle(filePath string) bool  { return filepath.Ext(filePath) == ".go" }
func (worktreeTestExtractor) IsEntryPoint(*model.Symbol) bool { return false }
func (worktreeTestExtractor) Extract(filePath string) ([]*model.Contract, error) {
	if _, err := findUpward(filePath, "go.mod"); err != nil {
		return nil, err
	}
	c := &model.Contract{
		Kind:       model.ContractKindCLICommand,
		Identifier: "test",
		File:       filePath,
		Line:       1,
		Signature:  map[string]interface{}{"use": "test"},
		Framework:  "cobra",
		Language:   model.LanguageGo,
	}
	if err := c.ComputeSignatureHash(); err != nil {
		return nil, err
	}
	return []*model.Contract{c}, nil
}

func TestCreateTempWorktree_Basic(t *testing.T) {
	repo := initGitRepo(t)
	worktreeDir, cleanup, err := createTempWorktree(repo, "HEAD")
	if err != nil {
		t.Fatalf("createTempWorktree failed: %v", err)
	}
	if _, err := os.Stat(worktreeDir); err != nil {
		t.Fatalf("worktree dir does not exist: %v", err)
	}
	cleanup()
	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		t.Fatalf("worktree dir still exists after cleanup: %v", err)
	}
}

func TestLoadContractsAtRef_PreservesGoMod(t *testing.T) {
	repo := initGitRepo(t)
	contracts, err := LoadContractsAtRef(repo, "HEAD", []string{"cmd/root.go"}, []ContractExtractor{worktreeTestExtractor{}})
	if err != nil {
		t.Fatalf("LoadContractsAtRef failed: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}
}

func TestLoadContractsAtRef_FilePathIsRelative(t *testing.T) {
	repo := initGitRepo(t)
	contracts, err := LoadContractsAtRef(repo, "HEAD", []string{"cmd/root.go"}, []ContractExtractor{worktreeTestExtractor{}})
	if err != nil {
		t.Fatalf("LoadContractsAtRef failed: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}
	if contracts[0].File != "cmd/root.go" {
		t.Fatalf("contract.File = %q, want %q", contracts[0].File, "cmd/root.go")
	}
	if filepath.IsAbs(contracts[0].File) {
		t.Fatalf("contract.File should be relative, got %q", contracts[0].File)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustWriteFile(t, filepath.Join(repo, "go.mod"), "module example.com/worktree\n\ngo 1.22\n")
	mustWriteFile(t, filepath.Join(repo, "cmd", "root.go"), `package cmd

func NewRoot() {}
`)
	runGit(t, repo, "init")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	return repo
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func findUpward(start, name string) (string, error) {
	dir := filepath.Dir(start)
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
