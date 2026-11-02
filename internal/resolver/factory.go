package resolver

import (
	"path/filepath"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ResolverFactory creates appropriate resolvers based on language and project root
type ResolverFactory struct {
	projectRoot string
}

// NewResolverFactory creates a new resolver factory
func NewResolverFactory(projectRoot string) *ResolverFactory {
	return &ResolverFactory{
		projectRoot: projectRoot,
	}
}

// GetResolver returns the appropriate resolver for the given language
func (f *ResolverFactory) GetResolver(lang model.Language) ImportResolver {
	switch lang {
	case model.LanguageGo:
		return NewGoResolver(f.projectRoot)
	case model.LanguageJavaScript, model.LanguageTypeScript:
		return NewJSResolver(f.projectRoot)
	case model.LanguageJava:
		return NewJavaResolver(f.projectRoot)
	case model.LanguagePython:
		return NewPythonResolver(f.projectRoot)
	default:
		return nil
	}
}

// ResolveImportsForFile resolves all imports in a file to actual file paths
// It takes the file path, language, and list of import strings
// Returns a map of import string -> resolved file path
func (f *ResolverFactory) ResolveImportsForFile(filePath string, lang model.Language, imports []string) map[string]string {
	resolver := f.GetResolver(lang)
	if resolver == nil {
		return nil
	}

	resolved := make(map[string]string)
	for _, imp := range imports {
		if resolvedPath, err := resolver.ResolveImport(imp, filePath); err == nil {
			resolved[imp] = resolvedPath
		}
		// Silently skip imports that can't be resolved (e.g., node_modules)
	}

	return resolved
}

// FindProjectRoot attempts to find the project root by looking for marker files
// It walks up the directory tree from the given file path
func FindProjectRoot(filePath string) string {
	// Markers that indicate project root
	markers := []string{
		"package.json",   // JavaScript/TypeScript
		"go.mod",         // Go
		"pom.xml",        // Maven
		"build.gradle",   // Gradle
		"pyproject.toml", // Python
		"setup.py",       // Python
		".git",           // Git repository
	}

	dir := filepath.Dir(filePath)
	for {
		// Check if any marker exists in current directory
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker)
			if fileExists(markerPath) || dirExists(markerPath) {
				return dir
			}
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, return original directory
			return filepath.Dir(filePath)
		}
		dir = parent
	}
}
