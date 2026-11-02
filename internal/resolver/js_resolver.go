package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JSResolver handles import resolution for JavaScript and TypeScript files
type JSResolver struct {
	// ProjectRoot is the root directory of the project
	ProjectRoot string
}

// NewJSResolver creates a new JavaScript/TypeScript import resolver
func NewJSResolver(projectRoot string) *JSResolver {
	return &JSResolver{
		ProjectRoot: projectRoot,
	}
}

// ResolveImport resolves a JavaScript/TypeScript import to an actual file path
// Handles:
// - Relative imports: ./file, ../file
// - File extensions: .js, .ts, .jsx, .tsx, .mjs, .cjs
// - Index files: index.js, index.ts, etc.
func (r *JSResolver) ResolveImport(importPath string, fromFile string) (string, error) {
	// Only handle relative imports (node_modules resolution is complex and skipped for MVP)
	if !strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "../") {
		return "", fmt.Errorf("non-relative imports not supported in minimal resolver: %s", importPath)
	}

	// Get the directory of the importing file
	fromDir := filepath.Dir(fromFile)

	// Resolve the import path relative to the importing file
	basePath := filepath.Join(fromDir, importPath)

	// Try to find the actual file with various extensions
	resolvedPath, err := r.findJSFile(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve import %q from %q: %w", importPath, fromFile, err)
	}

	return resolvedPath, nil
}

// findJSFile tries to find a JavaScript/TypeScript file with various extensions
// Also handles directory imports by looking for index files
func (r *JSResolver) findJSFile(basePath string) (string, error) {
	// Clean the path
	basePath = filepath.Clean(basePath)

	// List of extensions to try, in order of preference
	extensions := []string{
		".ts",   // TypeScript
		".tsx",  // TypeScript JSX
		".js",   // JavaScript
		".jsx",  // JavaScript JSX
		".mjs",  // ES Module
		".cjs",  // CommonJS
		".d.ts", // TypeScript declaration
	}

	// First, try the exact path with each extension
	for _, ext := range extensions {
		filePath := basePath + ext
		if fileExists(filePath) {
			return filePath, nil
		}
	}

	// If no file found, check if it's a directory and try index files
	if dirExists(basePath) {
		for _, ext := range extensions {
			indexPath := filepath.Join(basePath, "index"+ext)
			if fileExists(indexPath) {
				return indexPath, nil
			}
		}
	}

	return "", fmt.Errorf("file not found: %s", basePath)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
