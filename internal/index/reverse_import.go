package index

import (
	"strings"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ReverseImportIndex maintains bidirectional import relationships
// to efficiently answer "which files import this module?"
type ReverseImportIndex struct {
	mu sync.RWMutex

	// importedBy maps module/package name to files that import it
	// e.g., "fmt" -> ["main.go", "utils.go"]
	// Besides exact import strings, class-level imports are also filed under
	// their package prefix (see AddFileImports).
	importedBy map[string][]string

	// imports maps file to modules/packages it imports
	// e.g., "main.go" -> ["fmt", "strings", "./utils"]
	imports map[string][]string
}

// NewReverseImportIndex creates a new reverse import index
func NewReverseImportIndex() *ReverseImportIndex {
	return &ReverseImportIndex{
		importedBy: make(map[string][]string),
		imports:    make(map[string][]string),
	}
}

// AddFileImports records imports for a file.
//
// The reverse index keys each import twice when the string looks like a
// class-level import: under the exact import string and under its package
// prefix ("com.example.lib.Helper" also files under "com.example.lib"), so
// that queries keyed by a package clause match them. The detection rule in
// importPackagePrefix is deliberately shape-based and conservative; Go
// module paths, JS/TS specifiers and lowercase-tailed dotted imports keep
// their existing single-key behavior.
func (r *ReverseImportIndex) AddFileImports(filePath string, imports []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store forward imports
	r.imports[filePath] = imports

	// Update reverse index
	for _, imp := range imports {
		if !contains(r.importedBy[imp], filePath) {
			r.importedBy[imp] = append(r.importedBy[imp], filePath)
		}
		if pkg := importPackagePrefix(imp); pkg != "" && !contains(r.importedBy[pkg], filePath) {
			r.importedBy[pkg] = append(r.importedBy[pkg], filePath)
		}
	}
}

// RemoveFile removes a file from the index
func (r *ReverseImportIndex) RemoveFile(filePath string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get imports for this file
	imports := r.imports[filePath]

	// Remove from reverse index (exact keys and derived package-prefix keys)
	for _, imp := range imports {
		r.importedBy[imp] = removeString(r.importedBy[imp], filePath)
		if len(r.importedBy[imp]) == 0 {
			delete(r.importedBy, imp)
		}
		if pkg := importPackagePrefix(imp); pkg != "" {
			r.importedBy[pkg] = removeString(r.importedBy[pkg], filePath)
			if len(r.importedBy[pkg]) == 0 {
				delete(r.importedBy, pkg)
			}
		}
	}

	// Remove forward index
	delete(r.imports, filePath)
}

// GetImporters returns all files that import the given module/package
func (r *ReverseImportIndex) GetImporters(moduleName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.importedBy[moduleName]))
	copy(result, r.importedBy[moduleName])
	return result
}

// GetImports returns all imports for the given file
func (r *ReverseImportIndex) GetImports(filePath string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.imports[filePath]))
	copy(result, r.imports[filePath])
	return result
}

// Clear clears all data from the index
func (r *ReverseImportIndex) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.importedBy = make(map[string][]string)
	r.imports = make(map[string][]string)
}

// Stats returns statistics about the index
func (r *ReverseImportIndex) Stats() (fileCount, moduleCount int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.imports), len(r.importedBy)
}

// BuildFromFileSymbols builds the index from a collection of FileSymbols
func (r *ReverseImportIndex) BuildFromFileSymbols(files []*model.FileSymbols) {
	r.Clear()
	for _, file := range files {
		r.AddFileImports(file.File, file.Imports)
	}
}

// Helper functions

// importPackagePrefix returns the package prefix of a class-level import
// string ("com.example.lib.Helper" -> "com.example.lib"), or "" when the
// import is already a package-level key. The rule is shape-based and
// conservative — a prefix is derived only when the import is a dotted name
// whose last segment starts with an ASCII uppercase letter (Java's
// class-name convention; package segments are lowercase by convention):
//   - slash-bearing specifiers are never stripped: Go module paths
//     ("example.com/mod/lib") and JS/TS relative imports ("./lib") are
//     already package-level keys;
//   - lowercase-tailed dotted imports stay as-is (Python "lib.greet",
//     "os.path"): the tail is a member or submodule, not a class;
//   - Python class imports ("models.User" -> "models") match the same rule
//     and benefit from it the same way Java imports do.
func importPackagePrefix(imp string) string {
	if strings.Contains(imp, "/") {
		return ""
	}
	idx := strings.LastIndex(imp, ".")
	if idx <= 0 || idx == len(imp)-1 {
		return ""
	}
	if last := imp[idx+1]; last < 'A' || last > 'Z' {
		return ""
	}
	return imp[:idx]
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
