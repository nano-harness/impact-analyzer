package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PythonResolver struct{ projectRoot string }

func NewPythonResolver(projectRoot string) *PythonResolver {
	return &PythonResolver{projectRoot: projectRoot}
}

func (r *PythonResolver) ResolveImport(importPath string, fromFile string) (string, error) {
	if strings.TrimSpace(importPath) == "" {
		return "", fmt.Errorf("empty import")
	}
	if strings.Contains(importPath, "importlib") {
		return "", fmt.Errorf("dynamic import unresolved")
	}
	base := r.projectRoot
	if strings.HasPrefix(importPath, ".") {
		dotCount := 0
		for dotCount < len(importPath) && importPath[dotCount] == '.' {
			dotCount++
		}
		importPath = importPath[dotCount:]
		dir := filepath.Dir(fromFile)
		for i := 1; i < dotCount; i++ {
			dir = filepath.Dir(dir)
		}
		base = dir
	}
	parts := strings.Split(importPath, ".")
	candidates := []string{filepath.Join(append([]string{base}, append(parts, "__init__.py")...)...), filepath.Join(append([]string{base}, parts...)...) + ".py", filepath.Join(append([]string{base}, parts...)...) + ".pyi"}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not resolve Python import %q from %s", importPath, fromFile)
}
