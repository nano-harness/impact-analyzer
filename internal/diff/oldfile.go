package diff

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// OldFileLoader loads and parses file symbols from the base ref.
type OldFileLoader struct {
	repoPath string
	ref      string
	refErr   error
	cache    map[string]*model.FileSymbols
}

func NewOldFileLoader(repoPath, ref string) *OldFileLoader {
	loader := &OldFileLoader{repoPath: repoPath, ref: ref, cache: make(map[string]*model.FileSymbols)}
	if ref == "" {
		return loader
	}
	resolved, err := NewGit(repoPath).ResolveRef(ref)
	if err != nil {
		loader.refErr = err
		return loader
	}
	loader.ref = resolved
	return loader
}

func (l *OldFileLoader) LoadOldSymbols(oldPath string) (*model.FileSymbols, error) {
	if fs, ok := l.cache[oldPath]; ok {
		return fs, nil
	}
	content, err := l.LoadOldContent(oldPath)
	if err != nil {
		return nil, err
	}

	language := lang.DetectLanguage(oldPath)
	if language == model.LanguageUnknown {
		return nil, fmt.Errorf("unknown language for %s", oldPath)
	}
	adapter, err := lang.Get(language)
	if err != nil {
		return nil, err
	}

	// Preserve the original basename: adapters that derive package/module
	// names from the file path (e.g. Python's module name) must see the same
	// basename as the worktree parse, otherwise old/new symbols of one file
	// get different qualified names and a modify splits into delete+add.
	tmpDir, err := os.MkdirTemp("", "impact-old-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, filepath.Base(oldPath))
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return nil, err
	}

	fileSymbols, err := adapter.ParseFile(tmpPath)
	if err != nil {
		return nil, err
	}
	normalizeSymbolFiles(fileSymbols, oldPath)
	l.cache[oldPath] = fileSymbols
	return fileSymbols, nil
}

// LoadOldContent returns the raw file content at the base ref.
func (l *OldFileLoader) LoadOldContent(oldPath string) ([]byte, error) {
	if oldPath == "" {
		return nil, fmt.Errorf("empty old path")
	}
	if l.refErr != nil {
		return nil, fmt.Errorf("failed to resolve git ref %q: %w", l.ref, l.refErr)
	}
	if strings.HasPrefix(l.ref, "-") {
		return nil, fmt.Errorf("invalid git ref %q: refs must not start with '-'", l.ref)
	}

	// Paths are repoPath-relative (git diff --relative); the :./ prefix tells
	// git to interpret the path relative to the current working directory
	// (cmd.Dir = repoPath) rather than the repository root.
	cmd := exec.Command("git", "show", l.ref+":./"+oldPath)
	cmd.Dir = l.repoPath
	content, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s failed: %w", l.ref, oldPath, err)
	}
	return content, nil
}
