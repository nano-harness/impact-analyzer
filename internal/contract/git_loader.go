package contract

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// LoadContractsAtRef loads contracts from files at a specific git ref
// If ref is empty, loads from the current working tree
func LoadContractsAtRef(repoRoot string, ref string, files []string, extractors []ContractExtractor) ([]*model.Contract, error) {
	if len(files) == 0 || len(extractors) == 0 {
		return []*model.Contract{}, nil
	}

	// If ref is empty, read from working tree directly
	if ref == "" {
		return loadContractsFromWorkingTree(repoRoot, files, extractors)
	}

	worktreeDir, cleanup, err := createTempWorktree(repoRoot, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree at ref %s: %w", ref, err)
	}
	defer cleanup()

	// files are repoRoot-relative, but the worktree is a checkout of the
	// whole git repository. When repoRoot is a subdirectory of the git
	// root, translate paths into worktree space and translate the
	// extracted contract files back afterwards.
	wtPrefix := worktreePrefix(repoRoot)
	loadFiles := files
	if wtPrefix != "" {
		loadFiles = make([]string, 0, len(files))
		for _, f := range files {
			if filepath.IsAbs(f) {
				loadFiles = append(loadFiles, f)
				continue
			}
			loadFiles = append(loadFiles, filepath.Join(wtPrefix, f))
		}
	}

	contracts, err := loadContractsFromWorkingTree(worktreeDir, loadFiles, extractors)
	if err != nil {
		return nil, err
	}
	for _, contract := range contracts {
		if filepath.IsAbs(contract.File) {
			if rel, err := filepath.Rel(worktreeDir, contract.File); err == nil {
				contract.File = rel
			}
		}
		if wtPrefix != "" && !filepath.IsAbs(contract.File) {
			if rel, err := filepath.Rel(wtPrefix, contract.File); err == nil && !strings.HasPrefix(rel, "..") {
				contract.File = rel
			}
		}
	}
	return contracts, nil
}

// worktreePrefix returns repoRoot's path relative to the git repository
// root, or "" when repoRoot is the root (or the root cannot be determined).
func worktreePrefix(repoRoot string) string {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return ""
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = repoRoot
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	top, err := filepath.Abs(strings.TrimSpace(string(out)))
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(top, absRepo)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

func relativizeContractFiles(repoRoot string, contracts []*model.Contract) {
	for _, c := range contracts {
		if c == nil || c.File == "" || !filepath.IsAbs(c.File) {
			continue
		}
		if rel, err := filepath.Rel(repoRoot, c.File); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			c.File = rel
		}
	}
}

func MatchedExtractorNames(repoRoot string, files []string, extractors []ContractExtractor) []string {
	seen := map[string]bool{}
	var names []string
	for _, file := range files {
		filePath := file
		if !filepath.IsAbs(file) {
			filePath = filepath.Join(repoRoot, file)
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}
		for _, extractor := range extractors {
			if extractor.CanHandle(filePath) && !seen[extractor.Name()] {
				seen[extractor.Name()] = true
				names = append(names, extractor.Name())
			}
		}
	}
	return names
}

func ExtractorNames(extractors []ContractExtractor) []string {
	names := make([]string, 0, len(extractors))
	for _, extractor := range extractors {
		names = append(names, extractor.Name())
	}
	return names
}

func loadContractsFromWorkingTree(repoRoot string, files []string, extractors []ContractExtractor) ([]*model.Contract, error) {
	// Extraction batches must not interleave: a batch's deferred
	// ClearParseCache closes trees another batch could still be walking.
	extractBatchMu.Lock()
	defer extractBatchMu.Unlock()
	// The tree-sitter based extractors share one cached parse per file for
	// the duration of this batch; release all cached trees when it ends.
	defer astutil.ClearParseCache()

	// Files are processed concurrently; per-file results merge in the
	// original order, so the contract sequence is identical to a serial pass.
	return mapFilesParallel(files, func(file string) []*model.Contract {
		filePath := file
		if !filepath.IsAbs(file) {
			filePath = filepath.Join(repoRoot, file)
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil
		}

		var fileContracts []*model.Contract
		// Find compatible extractors
		for _, extractor := range extractors {
			if !extractor.CanHandle(filePath) {
				continue
			}

			// Extract contracts
			contracts, err := extractor.Extract(filePath)
			if err != nil {
				// Log error but continue with other files
				continue
			}

			relativizeContractFiles(repoRoot, contracts)
			fileContracts = append(fileContracts, contracts...)
		}
		return fileContracts
	}), nil
}

func createTempWorktree(repoRoot, ref string) (string, func(), error) {
	parentDir, err := os.MkdirTemp("", "impact-worktree-*")
	if err != nil {
		return "", nil, err
	}
	tmpDir := filepath.Join(parentDir, "worktree")

	cmd := exec.Command("git", "worktree", "add", "--detach", "--quiet", tmpDir, ref)
	cmd.Dir = repoRoot
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		os.RemoveAll(parentDir)
		return "", nil, fmt.Errorf("git worktree add failed: %w", err)
	}

	cleanup := func() {
		rm := exec.Command("git", "worktree", "remove", "--force", tmpDir)
		rm.Dir = repoRoot
		rm.Stderr = io.Discard
		_ = rm.Run()
		_ = os.RemoveAll(parentDir)
	}

	return tmpDir, cleanup, nil
}

// LoadContractsFromFiles loads contracts from a list of files in the working tree
// This is a convenience wrapper around LoadContractsAtRef with empty ref
func LoadContractsFromFiles(repoRoot string, files []string, extractors []ContractExtractor) ([]*model.Contract, error) {
	return LoadContractsAtRef(repoRoot, "", files, extractors)
}

// LoadContractsForDiff loads contracts from both old (at ref) and new (working tree) versions
// Returns (oldContracts, newContracts, error)
func LoadContractsForDiff(repoRoot string, ref string, files []string, extractors []ContractExtractor) ([]*model.Contract, []*model.Contract, error) {
	// Load old version from ref
	oldContracts, err := LoadContractsAtRef(repoRoot, ref, files, extractors)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load contracts at ref %s: %w", ref, err)
	}

	// Load new version from working tree
	newContracts, err := LoadContractsFromFiles(repoRoot, files, extractors)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load contracts from working tree: %w", err)
	}

	return oldContracts, newContracts, nil
}
