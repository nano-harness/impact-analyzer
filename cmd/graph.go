package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	langpkg "github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/nano-harness/impact-analyzer/internal/resolver"
	"github.com/spf13/cobra"
)

var (
	graphSymbol  string
	graphFile    string
	graphDepth   int
	graphReverse bool
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Visualize call graph or dependency graph",
	Long: `Generate and visualize call graphs or dependency graphs for symbols or files.

Examples:
  # Show callers of a specific function
  impact graph --symbol "pkg.FunctionName"

  # Show dependency graph for a file
  impact graph --file path/to/file.go

  # Limit depth and output as Mermaid
  impact graph --symbol "pkg.Func" --depth 3 --format mermaid`,
	RunE: runGraph,
}

func init() {
	rootCmd.AddCommand(graphCmd)

	graphCmd.Flags().StringVarP(&graphSymbol, "symbol", "s", "", "symbol to analyze (e.g., pkg.FunctionName)")
	graphCmd.Flags().StringVar(&graphFile, "file", "", "file to analyze")
	graphCmd.Flags().IntVar(&graphDepth, "depth", 3, "maximum graph depth")
	graphCmd.Flags().BoolVar(&graphReverse, "reverse", false, "show reverse dependencies (what depends on this file)")

	graphCmd.MarkFlagsMutuallyExclusive("symbol", "file")
}

func runGraph(cmd *cobra.Command, args []string) error {
	if err := requireAdapters(); err != nil {
		return err
	}

	if graphSymbol == "" && graphFile == "" {
		return fmt.Errorf("either --symbol or --file must be specified")
	}

	orch, cleanup, err := newOrchestrator(nil) // graph doesn't need excludes from CLI
	if err != nil {
		return err
	}
	defer cleanup()

	defer timedStep("graph generation")()

	formatValue := strings.ToLower(format)
	depth := effectiveDepth(cmd, graphDepth, orch.Config())

	if graphSymbol != "" {
		callGraph, err := orch.BuildRepoCallGraph()
		if err != nil {
			return err
		}

		target := findSymbol(callGraph, graphSymbol)
		if target == nil {
			return fmt.Errorf("symbol %q not found in repository call graph", graphSymbol)
		}

		output, err := renderCallGraph(callGraph, target, depth, formatValue)
		if err != nil {
			return err
		}
		fmt.Println(output)
		return nil
	}

	output, err := renderDependencyGraph(graphFile, depth, graphReverse, formatValue)
	if err != nil {
		return err
	}
	fmt.Println(output)

	return nil
}

func findSymbol(cg *graph.CallGraph, name string) *model.Symbol {
	if sym, ok := cg.GetSymbol(name); ok {
		return sym
	}

	for _, sym := range cg.Symbols() {
		if sym == nil {
			continue
		}
		if sym.QualifiedName() == name || sym.Name == name {
			return sym
		}
	}
	return nil
}

func renderCallGraph(cg *graph.CallGraph, root *model.Symbol, depth int, formatValue string) (string, error) {
	switch formatValue {
	case "mermaid":
		return renderCallGraphMermaid(cg, root, depth), nil
	case "text", "":
		return renderCallGraphText(cg, root, depth), nil
	default:
		return "", fmt.Errorf("unsupported format for graph: %s", formatValue)
	}
}

func renderCallGraphText(cg *graph.CallGraph, root *model.Symbol, depth int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Call graph for %s (depth %d)\n", root.QualifiedName(), depth))

	type queueItem struct {
		sym   *model.Symbol
		level int
	}
	queue := []queueItem{{sym: root, level: 0}}
	visited := map[string]bool{root.QualifiedName(): true}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		callers := cg.GetCallers(item.sym)
		for _, caller := range callers {
			if caller == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s%s -> %s\n",
				strings.Repeat("  ", item.level),
				caller.QualifiedName(),
				item.sym.QualifiedName()))

			if depth >= 0 && item.level+1 >= depth {
				continue
			}

			if !visited[caller.QualifiedName()] {
				visited[caller.QualifiedName()] = true
				queue = append(queue, queueItem{sym: caller, level: item.level + 1})
			}
		}
	}

	return sb.String()
}

func renderCallGraphMermaid(cg *graph.CallGraph, root *model.Symbol, depth int) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("graph TD\n")

	type queueItem struct {
		sym   *model.Symbol
		level int
	}
	queue := []queueItem{{sym: root, level: 0}}
	visited := map[string]bool{root.QualifiedName(): true}
	seenEdge := make(map[string]bool)

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		callers := cg.GetCallers(item.sym)
		for _, caller := range callers {
			if caller == nil {
				continue
			}

			from := sanitizeNode(caller.QualifiedName())
			to := sanitizeNode(item.sym.QualifiedName())
			edgeKey := from + "->" + to
			if !seenEdge[edgeKey] {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
				seenEdge[edgeKey] = true
			}

			if depth >= 0 && item.level+1 >= depth {
				continue
			}

			if !visited[caller.QualifiedName()] {
				visited[caller.QualifiedName()] = true
				queue = append(queue, queueItem{sym: caller, level: item.level + 1})
			}
		}
	}

	sb.WriteString("```\n")
	return sb.String()
}

func renderDependencyGraph(filePath string, depth int, reverse bool, formatValue string) (string, error) {
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		var err error
		absPath, err = filepath.Abs(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve file path: %w", err)
		}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat %s: %w", filePath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory; please provide a file", filePath)
	}

	language := langpkg.DetectLanguage(absPath)
	if language == model.LanguageUnknown {
		return "", fmt.Errorf("unsupported language for dependency graph: %s", filePath)
	}

	adapter, err := langpkg.Get(language)
	if err != nil {
		return "", err
	}

	// Build dependency map
	depMap := make(map[string][]string)
	visited := make(map[string]bool)

	if reverse {
		// For reverse dependencies, we need to scan the repository
		if err := buildReverseDependencyMap(absPath, depMap); err != nil {
			return "", fmt.Errorf("failed to build reverse dependency map: %w", err)
		}
	} else {
		// For forward dependencies, recursively resolve imports
		if err := buildForwardDependencyMap(absPath, adapter, depMap, visited, depth); err != nil {
			return "", fmt.Errorf("failed to build dependency map: %w", err)
		}
	}

	switch formatValue {
	case "json":
		return renderDependencyJSON(absPath, depMap, reverse)
	case "mermaid":
		return renderDependencyMermaid(absPath, depMap, reverse), nil
	case "text", "":
		return renderDependencyText(absPath, depMap, reverse), nil
	default:
		return "", fmt.Errorf("unsupported format for graph: %s", formatValue)
	}
}

func buildForwardDependencyMap(filePath string, adapter langpkg.LanguageAdapter, depMap map[string][]string, visited map[string]bool, depth int) error {
	if depth == 0 || visited[filePath] {
		return nil
	}
	visited[filePath] = true

	imports, err := adapter.ResolveImports(filePath)
	if err != nil {
		return err
	}

	// Find project root for import resolution
	projectRoot := resolver.FindProjectRoot(filePath)
	factory := resolver.NewResolverFactory(projectRoot)
	language := adapter.Language()

	// Resolve import paths to actual file paths
	resolvedImports := factory.ResolveImportsForFile(filePath, language, imports)

	// Store resolved file paths in depMap
	var resolvedPaths []string
	for _, resolvedPath := range resolvedImports {
		resolvedPaths = append(resolvedPaths, resolvedPath)
	}
	depMap[filePath] = resolvedPaths

	// Recursively resolve transitive dependencies if depth allows
	if depth > 1 {
		for _, resolvedPath := range resolvedPaths {
			if !visited[resolvedPath] {
				// Recursively build dependency map for resolved file
				if err := buildForwardDependencyMap(resolvedPath, adapter, depMap, visited, depth-1); err != nil {
					// Silently skip files that fail to parse (e.g., external dependencies)
					continue
				}
			}
		}
	}

	return nil
}

func buildReverseDependencyMap(targetFile string, depMap map[string][]string) error {
	// Walk the repository to find files that import the target
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	targetAbsPath, err := filepath.Abs(targetFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// For Go files, map the target file to its package import path
	targetPkgPath, err := fileToPackagePath(targetAbsPath)
	if err != nil {
		// If we can't determine package path, fall back to string matching
		targetPkgPath = ""
	}

	// Create resolver factory
	projectRoot := resolver.FindProjectRoot(targetAbsPath)
	factory := resolver.NewResolverFactory(projectRoot)

	var reverseDeps []string
	err = filepath.WalkDir(wd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "dist", "build", ".impact":
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the target file itself
		absPath, _ := filepath.Abs(path)
		if absPath == targetAbsPath {
			return nil
		}

		language := langpkg.DetectLanguage(path)
		if language == model.LanguageUnknown {
			return nil
		}

		adapter, err := langpkg.Get(language)
		if err != nil {
			return nil
		}

		imports, err := adapter.ResolveImports(path)
		if err != nil {
			return nil
		}

		// For Go files: check if any import matches the target package path
		if language == model.LanguageGo && targetPkgPath != "" {
			for _, imp := range imports {
				if imp == targetPkgPath {
					reverseDeps = append(reverseDeps, path)
					break
				}
			}
			return nil
		}

		// For other languages: use language-specific resolver
		langResolver := factory.GetResolver(language)
		if langResolver == nil {
			// Fallback to string matching for languages without resolver
			targetBase := filepath.Base(targetFile)
			for _, imp := range imports {
				if strings.Contains(imp, targetBase) {
					reverseDeps = append(reverseDeps, path)
					break
				}
			}
			return nil
		}

		// For non-Go languages, use the resolver
		targetDir := filepath.Dir(targetAbsPath)
		for _, imp := range imports {
			resolvedPath, err := langResolver.ResolveImport(imp, path)
			if err != nil {
				continue
			}

			// Check if the resolved path matches the target directory or file
			if resolvedPath == targetDir || resolvedPath == targetAbsPath {
				reverseDeps = append(reverseDeps, path)
				break
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	depMap[targetFile] = reverseDeps
	return nil
}

// fileToPackagePath maps a Go file to its package import path
// e.g., /path/to/repo/internal/contract/diff.go -> github.com/user/repo/internal/contract
func fileToPackagePath(filePath string) (string, error) {
	language := langpkg.DetectLanguage(filePath)
	if language != model.LanguageGo {
		return "", fmt.Errorf("not a Go file")
	}

	// Find the directory containing the file
	dir := filepath.Dir(filePath)

	// Find the module root by looking for go.mod
	moduleRoot := dir
	for {
		if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			// Reached filesystem root without finding go.mod
			return "", fmt.Errorf("go.mod not found")
		}
		moduleRoot = parent
	}

	// Read go.mod to get module name
	goModPath := filepath.Join(moduleRoot, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	// Parse module name from go.mod (first line should be "module <name>")
	lines := strings.Split(string(goModContent), "\n")
	var moduleName string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			break
		}
	}

	if moduleName == "" {
		return "", fmt.Errorf("module name not found in go.mod")
	}

	// Calculate relative path from module root to file directory
	relPath, err := filepath.Rel(moduleRoot, dir)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	// Construct package import path
	if relPath == "." {
		return moduleName, nil
	}

	// Convert Windows-style paths to forward slashes
	relPath = filepath.ToSlash(relPath)

	return moduleName + "/" + relPath, nil
}

func renderDependencyText(filePath string, depMap map[string][]string, reverse bool) string {
	var sb strings.Builder

	if reverse {
		sb.WriteString(fmt.Sprintf("Reverse Dependencies for %s\n", filePath))
		sb.WriteString(fmt.Sprintf("(Files that depend on %s)\n\n", filepath.Base(filePath)))
	} else {
		sb.WriteString(fmt.Sprintf("Dependencies for %s\n\n", filePath))
	}

	// For forward dependencies, show all files in depMap (transitive)
	if !reverse && len(depMap) > 1 {
		sb.WriteString(fmt.Sprintf("Total files: %d (including transitive dependencies)\n\n", len(depMap)))

		i := 1
		for file, deps := range depMap {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i, file))
			if len(deps) > 0 {
				sb.WriteString(fmt.Sprintf("   Imports: %d files\n", len(deps)))
				for _, dep := range deps {
					sb.WriteString(fmt.Sprintf("   - %s\n", dep))
				}
			}
			sb.WriteString("\n")
			i++
		}
		return sb.String()
	}

	// For reverse or simple case, show direct dependencies only
	deps := depMap[filePath]
	if len(deps) == 0 {
		sb.WriteString("No dependencies detected.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Total: %d dependencies\n\n", len(deps)))
	for i, dep := range deps {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, dep))
	}

	return sb.String()
}

func renderDependencyMermaid(filePath string, depMap map[string][]string, reverse bool) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("graph ")
	if reverse {
		sb.WriteString("BT\n") // Bottom to Top for reverse deps
	} else {
		sb.WriteString("TD\n") // Top to Down for forward deps
	}

	// For forward dependencies, render all edges in the transitive graph
	if !reverse && len(depMap) > 1 {
		seenEdge := make(map[string]bool)

		for file, deps := range depMap {
			fileNode := sanitizeNode(file)

			if len(deps) == 0 {
				// Ensure isolated nodes are shown
				sb.WriteString(fmt.Sprintf("    %s\n", fileNode))
				continue
			}

			for _, dep := range deps {
				depNode := sanitizeNode(dep)
				edgeKey := fileNode + "->" + depNode

				if !seenEdge[edgeKey] {
					sb.WriteString(fmt.Sprintf("    %s --> %s\n", fileNode, depNode))
					seenEdge[edgeKey] = true
				}
			}
		}

		sb.WriteString("```\n")
		return sb.String()
	}

	// For reverse dependencies, show simple graph
	root := sanitizeNode(filePath)
	deps := depMap[filePath]

	if len(deps) == 0 {
		sb.WriteString(fmt.Sprintf("    %s\n", root))
		sb.WriteString("```\n")
		return sb.String()
	}

	for _, dep := range deps {
		depNode := sanitizeNode(dep)
		if reverse {
			// Reverse: dep -> root (dependencies point to root)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", depNode, root))
		} else {
			// Forward: root -> dep (root depends on dep)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", root, depNode))
		}
	}

	sb.WriteString("```\n")
	return sb.String()
}

func sanitizeNode(name string) string {
	// Replace special characters that could break Mermaid syntax
	// Mermaid node IDs can contain alphanumeric, underscores, hyphens
	// Use a hash-based approach for complex names while keeping readability

	// First escape quotes
	safe := strings.ReplaceAll(name, "\"", "'")

	// Replace problematic characters for Mermaid with underscores
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		".", "_",
		":", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		"<", "_",
		">", "_",
		",", "_",
		";", "_",
		" ", "_",
	)
	safe = replacer.Replace(safe)

	// Wrap in quotes for display label
	return fmt.Sprintf("\"%s\"", safe)
}
