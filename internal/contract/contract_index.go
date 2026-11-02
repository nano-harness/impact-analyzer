package contract

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ContractIndex maintains a reverse index from handler symbols to contracts
// Enables efficient lookup of contracts by handler qualified name
type ContractIndex struct {
	byHandler map[string][]*model.Contract // handlerQName -> contracts
	all       []*model.Contract
}

// BuildIndex scans repoRoot for all supported files and extracts contracts
// using all registered extractors, building a handler symbol -> contracts index
func BuildIndex(repoRoot string, excludePaths []string) (*ContractIndex, error) {
	// Extraction batches must not interleave: a batch's deferred
	// ClearParseCache closes trees another batch could still be walking.
	extractBatchMu.Lock()
	defer extractBatchMu.Unlock()
	// Same-language extractors share one cached tree-sitter parse per file
	// during the build; release all cached trees when the build ends.
	defer astutil.ClearParseCache()

	verbose := os.Getenv("IMPACT_VERBOSE") != ""

	if verbose {
		fmt.Fprintf(os.Stderr, "[ContractIndex] Building index for %s\n", repoRoot)
	}

	// Get all registered extractors
	extractors := All()
	if len(extractors) == 0 {
		return &ContractIndex{
			byHandler: make(map[string][]*model.Contract),
			all:       []*model.Contract{},
		}, nil
	}

	// Collect all supported files.
	// Each file is read at most once per build; the content is handed to
	// extractors implementing ContentExtractor and cached for the extract
	// phase, so extractors never re-read the same file from disk.
	var files []string
	contents := make(map[string][]byte)
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			// Check if directory should be excluded
			relPath, _ := filepath.Rel(repoRoot, path)
			for _, pattern := range excludePaths {
				pattern = strings.TrimSuffix(pattern, "/**")
				pattern = strings.TrimSuffix(pattern, "/*")
				if strings.HasPrefix(relPath, pattern+"/") || relPath == pattern {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file should be excluded
		relPath, _ := filepath.Rel(repoRoot, path)
		for _, pattern := range excludePaths {
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				if strings.HasPrefix(relPath, prefix+"/") {
					return nil
				}
			}
		}

		// Check if any extractor can handle this file
		language := lang.DetectLanguage(path)
		if language == model.LanguageUnknown {
			return nil
		}

		// Read the file once and let every extractor inspect the content
		// instead of each extractor re-reading the file on its own.
		// On read failure, fall back to the legacy path-based CanHandle
		// so behavior matches the previous per-extractor reads exactly.
		content, readErr := os.ReadFile(path)
		for _, extractor := range extractors {
			var handled bool
			if readErr == nil {
				handled = canHandleContent(extractor, path, content)
			} else {
				handled = extractor.CanHandle(path)
			}
			if handled {
				files = append(files, path)
				if readErr == nil {
					contents[path] = content
				}
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[ContractIndex] Found %d potentially relevant files\n", len(files))
	}

	// Extract contracts from all files, reusing the content read during the
	// walk phase (files without cached content had a read failure and keep
	// the legacy path-based behavior). Files are processed concurrently;
	// per-file results merge in the original walk order, so the contract
	// sequence is identical to a serial pass.
	allContracts := mapFilesParallel(files, func(file string) []*model.Contract {
		var fileContracts []*model.Contract
		content, hasContent := contents[file]
		for _, extractor := range extractors {
			var handled bool
			if hasContent {
				handled = canHandleContent(extractor, file, content)
			} else {
				handled = extractor.CanHandle(file)
			}
			if !handled {
				continue
			}

			var contracts []*model.Contract
			var err error
			if hasContent {
				contracts, err = extractContent(extractor, file, content)
			} else {
				contracts, err = extractor.Extract(file)
			}
			if err != nil {
				// Log but continue
				if verbose {
					fmt.Fprintf(os.Stderr, "[ContractIndex] Failed to extract from %s: %v\n", file, err)
				}
				continue
			}

			fileContracts = append(fileContracts, contracts...)
		}
		return fileContracts
	})

	if verbose {
		fmt.Fprintf(os.Stderr, "[ContractIndex] Extracted %d contracts total\n", len(allContracts))
	}

	// Build reverse index: handler qname -> contracts
	byHandler := make(map[string][]*model.Contract)
	for _, contract := range allContracts {
		handlerQName := extractHandlerQName(contract)
		if handlerQName == "" {
			// No handler info available, skip indexing by handler
			continue
		}
		byHandler[handlerQName] = append(byHandler[handlerQName], contract)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[ContractIndex] Indexed %d unique handlers\n", len(byHandler))
	}

	return &ContractIndex{
		byHandler: byHandler,
		all:       allContracts,
	}, nil
}

// extractHandlerQName extracts handler qualified name from contract signature
// Different extractors store handler info differently in the signature
func extractHandlerQName(contract *model.Contract) string {
	if contract.Signature == nil {
		return ""
	}

	// Try common handler field names
	if handler, ok := contract.Signature["handler"].(string); ok {
		return handler
	}

	// For cobra: may store Run/RunE function name
	if runE, ok := contract.Signature["run_e"].(string); ok {
		return runE
	}
	if run, ok := contract.Signature["run"].(string); ok {
		return run
	}

	// For gRPC: might be service.method
	if service, ok := contract.Signature["service"].(string); ok {
		if method, ok := contract.Signature["method"].(string); ok {
			return service + "." + method
		}
	}

	return ""
}

// LookupByHandler returns all contracts that have the given symbol as a handler
func (ci *ContractIndex) LookupByHandler(sym *model.Symbol) []*model.Contract {
	if sym == nil {
		return nil
	}
	return ci.byHandler[sym.QualifiedName()]
}

// IsHandler checks if a symbol is a handler for any contract
// Useful as a TraversalOptions.IsStop predicate
func (ci *ContractIndex) IsHandler(sym *model.Symbol) bool {
	if sym == nil {
		return false
	}
	_, exists := ci.byHandler[sym.QualifiedName()]
	return exists
}

// AllHandlerQNames returns a set of all handler qualified names
func (ci *ContractIndex) AllHandlerQNames() map[string]bool {
	result := make(map[string]bool, len(ci.byHandler))
	for qname := range ci.byHandler {
		result[qname] = true
	}
	return result
}

// AllContracts returns all indexed contracts
func (ci *ContractIndex) AllContracts() []*model.Contract {
	return ci.all
}

// Count returns the number of contracts in the index
func (ci *ContractIndex) Count() int {
	return len(ci.all)
}

// HandlerCount returns the number of unique handlers in the index
func (ci *ContractIndex) HandlerCount() int {
	return len(ci.byHandler)
}
