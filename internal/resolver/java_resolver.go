package resolver

import (
	"fmt"
	"path/filepath"
	"strings"
)

// JavaResolver handles import resolution for Java files
type JavaResolver struct {
	// ProjectRoot is the root directory of the project
	ProjectRoot string
	// SourceRoots are the source directory roots (e.g., src/main/java, src/test/java)
	SourceRoots []string
}

// NewJavaResolver creates a new Java import resolver
// It assumes standard Maven/Gradle directory structure
func NewJavaResolver(projectRoot string) *JavaResolver {
	return &JavaResolver{
		ProjectRoot: projectRoot,
		SourceRoots: []string{
			"src/main/java",
			"src/test/java",
			"src",
		},
	}
}

// ResolveImport resolves a Java import to an actual file path
// Converts package notation (com.example.Service) to file path
// Example: com.example.Service -> src/main/java/com/example/Service.java
func (r *JavaResolver) ResolveImport(importPath string, fromFile string) (string, error) {
	// Convert package notation to file path
	// com.example.Service -> com/example/Service.java
	relativePath := strings.ReplaceAll(importPath, ".", string(filepath.Separator)) + ".java"

	// Try each source root
	for _, sourceRoot := range r.SourceRoots {
		fullPath := filepath.Join(r.ProjectRoot, sourceRoot, relativePath)
		if fileExists(fullPath) {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("Java class not found: %s (tried source roots: %v)", importPath, r.SourceRoots)
}

// WithSourceRoots allows customizing the source roots
func (r *JavaResolver) WithSourceRoots(roots []string) *JavaResolver {
	r.SourceRoots = roots
	return r
}
