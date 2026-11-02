package index

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// Manager handles index building and management
type Manager struct {
	store    IndexStore
	cacheDir string
}

// NewManager creates a new index manager
func NewManager(cacheDir string) (*Manager, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbPath := filepath.Join(cacheDir, "index.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		return nil, err
	}

	return &Manager{
		store:    store,
		cacheDir: cacheDir,
	}, nil
}

// IndexFile indexes a single file
// #15: Implements incremental build with file-level caching
func (m *Manager) IndexFile(filePath string) error {
	language := lang.DetectLanguage(filePath)
	if language == model.LanguageUnknown {
		return fmt.Errorf("unknown language for file: %s", filePath)
	}

	adapter, err := lang.Get(language)
	if err != nil {
		return err
	}

	// #15: Check if file needs re-parsing
	fileHash, err := computeFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to compute file hash: %w", err)
	}

	// Check if cached result is still valid
	cachedFileSymbols, err := m.store.GetFileSymbols(filePath)
	if err == nil && cachedFileSymbols != nil && cachedFileSymbols.ContentHash == fileHash {
		// File hasn't changed, use cached result
		return nil
	}

	// Parse symbols
	fileSymbols, err := adapter.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Store content hash for future cache checks
	fileSymbols.ContentHash = fileHash

	if err := m.store.StoreFileSymbols(fileSymbols); err != nil {
		return fmt.Errorf("failed to store symbols: %w", err)
	}

	// Extract calls
	calls, err := adapter.ExtractCalls(filePath)
	if err != nil {
		return fmt.Errorf("failed to extract calls: %w", err)
	}

	for _, call := range calls {
		if err := m.store.StoreCallSite(call); err != nil {
			return fmt.Errorf("failed to store call site: %w", err)
		}
	}

	return nil
}

// #15: computeFileHash computes SHA256 hash of file content
func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Close closes the index manager
func (m *Manager) Close() error {
	return m.store.Close()
}
