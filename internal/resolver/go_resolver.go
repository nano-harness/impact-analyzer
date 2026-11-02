package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoResolver resolves Go import paths to local file paths
type GoResolver struct {
	ProjectRoot string
	ModulePath  string // From go.mod, e.g. github.com/nano-harness/impact-analyzer
}

// NewGoResolver creates a new Go resolver
func NewGoResolver(projectRoot string) *GoResolver {
	modPath := readGoModule(projectRoot)
	return &GoResolver{
		ProjectRoot: projectRoot,
		ModulePath:  modPath,
	}
}

// ResolveImport resolves a Go import path to a directory absolute path
// For internal imports (matching module path prefix), returns the directory path
// For external/stdlib imports, returns error
// Note: Returns DIRECTORY path, not specific file (a Go package contains multiple .go files)
func (r *GoResolver) ResolveImport(importPath string, fromFile string) (string, error) {
	// Handle empty module path
	if r.ModulePath == "" {
		return "", fmt.Errorf("go.mod not found or module path not set")
	}

	// Check if this is an internal import (starts with module path)
	if !strings.HasPrefix(importPath, r.ModulePath) {
		return "", fmt.Errorf("external or stdlib import: %s", importPath)
	}

	// Remove module path prefix to get relative directory
	relDir := strings.TrimPrefix(importPath, r.ModulePath)
	relDir = strings.TrimPrefix(relDir, "/")

	// Convert to absolute path
	fullDir := filepath.Join(r.ProjectRoot, relDir)

	// Verify directory exists
	if !dirExists(fullDir) {
		return "", fmt.Errorf("directory not found: %s", fullDir)
	}

	return fullDir, nil
}

// readGoModule reads the module path from go.mod
func readGoModule(projectRoot string) string {
	goModPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	// Parse go.mod to find "module" directive
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			// Remove any comments or trailing content
			if idx := strings.IndexAny(modulePath, " \t//"); idx >= 0 {
				modulePath = strings.TrimSpace(modulePath[:idx])
			}
			return modulePath
		}
	}

	return ""
}
