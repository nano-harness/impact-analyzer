package contract

import (
	"path/filepath"
	"testing"
)

func TestLoadContractsFromWorkingTree_FilePathIsRelative(t *testing.T) {
	repo := initGitRepo(t)
	contracts, err := LoadContractsFromFiles(repo, []string{"cmd/root.go"}, []ContractExtractor{worktreeTestExtractor{}})
	if err != nil {
		t.Fatalf("LoadContractsFromFiles failed: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}
	if contracts[0].File != "cmd/root.go" {
		t.Fatalf("contract.File = %q, want cmd/root.go", contracts[0].File)
	}
	if filepath.IsAbs(contracts[0].File) {
		t.Fatalf("contract.File should be relative, got %q", contracts[0].File)
	}
}
