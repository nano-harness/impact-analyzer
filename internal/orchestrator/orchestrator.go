package orchestrator

import (
	"crypto/sha256"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/nano-harness/impact-analyzer/internal/analysis"
	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/diff"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/index"
	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/lang/golang"
	"github.com/nano-harness/impact-analyzer/internal/lang/golang/godeep"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/spf13/viper"
)

// defaultImportExpansionDepth is the default BFS depth for reverse import expansion.
// Matches the default in config.setDefaults().
const defaultImportExpansionDepth = 2

// Orchestrator coordinates the entire impact analysis workflow
type Orchestrator struct {
	repoPath       string
	cacheDir       string
	indexMgr       *index.Manager
	callGraph      *graph.CallGraph
	config         *config.Config
	excludePaths   []string // Paths to exclude from analysis (glob patterns)
	excludes       []excludePattern
	cache          *AnalysisCache
	reverseImports *index.ReverseImportIndex // #17-19: Reverse import index for incremental builds
	goModPath      string                    // cached go module path from go.mod
	version        string                    // tool version, mixed into analysis cache keys
	parseCache     *lang.ParseCache          // per-run memo for adapter.ParseFile results
	goDeepCache    *goDeepCallCache          // per-run memo for Go SSA/RTA package call graphs
	// Per-run shared Go SSA session state (lazy). The session replaces the
	// legacy per-directory LoadPackages+BuildSSA with a single load covering
	// the whole analysis root; the error is memoized so a failed session load
	// falls back to the legacy path once instead of being retried per pass.
	goDeepSession      *godeep.Session
	goDeepSessionErr   error
	goDeepSessionTried bool
	// goDeepSessionCh carries the result of an async session load kicked off
	// at the start of a call-graph build, so the expensive LoadPackages+SSA
	// build overlaps the tree-sitter parse loop instead of starting only when
	// the deep-analysis pass first needs it. Buffered (size 1): an abandoned
	// load (run reset or error path) never blocks its goroutine.
	goDeepSessionCh chan goDeepSessionResult
	// surfaceCh carries the result of surface-diff contract loading started
	// at the beginning of a diff run, so the git-worktree checkout and
	// contract extraction overlap the analysis pipeline. Same lifecycle as
	// goDeepSessionCh: set per run, drained once, buffered so an abandoned
	// load never blocks its goroutine.
	surfaceCh chan surfaceLoadResult
	// skipGoDeep disables the Go SSA/RTA edge pass in buildCallGraph for the
	// current repo-wide graph build. Set by the orphan/whylive flows, which
	// build their own analysis.SSASession for Go reachability right after;
	// without it those flows pay for a second full packages.Load + SSA build
	// whose edges the SSA session's reachability verdicts supersede.
	skipGoDeep bool
}

// goDeepSessionResult is the outcome of an async godeep.LoadSession.
type goDeepSessionResult struct {
	session *godeep.Session
	root    string
	err     error
}

// surfaceLoadResult is the outcome of an async contract.LoadContractsForDiff.
type surfaceLoadResult struct {
	oldContracts []*model.Contract
	newContracts []*model.Contract
	err          error
}

// excludePattern is an exclude pattern preprocessed once at construction so
// shouldExclude does not repeat per-call normalization work.
type excludePattern struct {
	pattern string // normalized (slash) pattern for glob matching
	prefix  string // literal prefix form (trailing "/**" or "/*" stripped)
	isGlob  bool   // whether pattern contains glob meta characters
}

// compileExcludePatterns normalizes exclude patterns for repeated matching.
// Patterns without glob meta characters can only ever match the exact literal
// string, which the prefix check already covers, so glob matching is skipped
// for them entirely.
func compileExcludePatterns(patterns []string) []excludePattern {
	excludes := make([]excludePattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		prefix := strings.TrimSuffix(pattern, "/**")
		prefix = strings.TrimSuffix(prefix, "/*")
		excludes = append(excludes, excludePattern{
			pattern: pattern,
			prefix:  prefix,
			isGlob:  strings.ContainsAny(pattern, `*?[{\`),
		})
	}
	return excludes
}

// Option customizes an Orchestrator at construction time.
type Option func(*Orchestrator)

// WithVersion sets the tool version mixed into analysis cache keys, so cached
// results from older builds are invalidated after a binary upgrade.
func WithVersion(version string) Option {
	return func(o *Orchestrator) {
		o.version = version
	}
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(repoPath, cacheDir string) (*Orchestrator, error) {
	return NewOrchestratorWithConfig(repoPath, cacheDir, nil)
}

// NewOrchestratorWithConfig creates a new orchestrator with provided configuration
func NewOrchestratorWithConfig(repoPath, cacheDir string, cfg *config.Config) (*Orchestrator, error) {
	return NewOrchestratorWithOptions(repoPath, cacheDir, cfg, nil)
}

// NewOrchestratorWithOptions creates a new orchestrator with configuration and CLI excludes
func NewOrchestratorWithOptions(repoPath, cacheDir string, cfg *config.Config, cliExcludes []string, opts ...Option) (*Orchestrator, error) {
	// Load default config if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			// Use empty config with defaults
			cfg = &config.Config{}
		}
	}

	// Use config cache dir if not specified
	if cacheDir == "" {
		if cfg.Index.CacheDir != "" {
			cacheDir = cfg.Index.CacheDir
		} else {
			cacheDir = filepath.Join(repoPath, ".impact/cache")
		}
	}

	indexMgr, err := index.NewManager(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create index manager: %w", err)
	}

	// Merge exclude paths from CLI, config, and defaults
	excludePaths := config.MergeExcludePaths(cliExcludes, cfg)

	// Initialize cache
	cache := NewAnalysisCache(cacheDir, cfg.Index.IncrementalMode)

	cg := graph.NewCallGraph()
	cg.SetPermissiveFallback(cfg.Callgraph.PermissiveFallback) // #15: Configure bare-name fallback

	o := &Orchestrator{
		repoPath:       repoPath,
		cacheDir:       cacheDir,
		indexMgr:       indexMgr,
		callGraph:      cg,
		config:         cfg,
		excludePaths:   excludePaths,
		excludes:       compileExcludePatterns(excludePaths),
		cache:          cache,
		reverseImports: index.NewReverseImportIndex(), // #17-19: Initialize reverse import index
		parseCache:     lang.NewParseCache(),
		goDeepCache:    newGoDeepCallCache(),
	}
	for _, opt := range opts {
		opt(o)
	}
	// Wire the initial graph to the reverse import index as well; all analyze
	// flows reset the graph before use, but keep the initial one consistent.
	o.callGraph.SetFileImportsFunc(o.fileImportsForGraph)
	return o, nil
}

// AnalyzeDiff performs complete impact analysis for a git diff
func (o *Orchestrator) AnalyzeDiff(diffRef string, maxDepth int) (*model.ImpactResult, error) {
	return o.AnalyzeDiffWithSurface(diffRef, maxDepth, true)
}

// AnalyzeDiffWithSurface performs complete impact analysis for a git diff with optional surface diff
func (o *Orchestrator) AnalyzeDiffWithSurface(diffRef string, maxDepth int, withSurface bool) (*model.ImpactResult, error) {
	// Compute the diff up front: the cache key must cover the working-tree
	// state, which the resolved ref and HEAD alone do not capture.
	gitOps := diff.NewGit(o.repoPath)
	diffText, changedFiles, err := gitOps.DiffWithChangedFiles(diffRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}
	diffHash := sha256.Sum256([]byte(diffText))

	// Check cache if incremental mode is enabled
	var cacheKey string
	verbose := os.Getenv("IMPACT_VERBOSE") != ""
	if o.config.Index.IncrementalMode && o.cache != nil {
		// Resolve ref
		resolvedRef, err := gitOps.ResolveRef(diffRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[cache] skip - failed to resolve ref %s: %v\n", diffRef, err)
		} else {
			// Get HEAD
			repoHEAD, err := gitOps.GetHEAD()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[cache] skip - failed to get HEAD: %v\n", err)
			} else {
				// Get config hash
				configPath := viper.ConfigFileUsed()
				configHash, err := ComputeConfigHash(configPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[cache] skip - failed to compute config hash: %v\n", err)
				} else {
					// Get version (fallback to "dev" if not set)
					version := o.version
					if version == "" {
						version = "dev"
					}

					// Compute cache key from every analysis-affecting parameter
					cacheKey = o.cache.ComputeKey(resolvedRef, repoHEAD, configHash, version, CacheKeyParams{
						MaxDepth:               maxDepth,
						StopAtEntryPoints:      o.config.Analysis.StopAtEntryPoints,
						UseDeepAnalysis:        o.config.Go.UseDeepAnalysis,
						TraceAffectedContracts: o.config.Analysis.TraceAffectedContracts,
						TraceMaxDepth:          o.config.Analysis.TraceMaxDepth,
						WithSurface:            withSurface,
						BuildTags:              o.config.Go.BuildTags,
						ContentHash:            fmt.Sprintf("%x", diffHash),
					})

					// Try to get from cache
					if cached, ok := o.cache.Get(cacheKey); ok {
						if verbose {
							fmt.Fprintf(os.Stderr, "[cache] hit: %s\n", cacheKey)
						}
						return cached, nil
					}
					if verbose {
						fmt.Fprintf(os.Stderr, "[cache] miss: %s\n", cacheKey)
					}
				}
			}
		}
	}

	// Perform the actual analysis
	result, err := o.analyzeDiffWithSurfaceInternal(diffRef, diffText, changedFiles, maxDepth, withSurface)
	if err != nil {
		return nil, err
	}

	// Store in cache if we have a valid key
	if cacheKey != "" && o.cache != nil {
		if err := o.cache.Put(cacheKey, result); err != nil {
			fmt.Fprintf(os.Stderr, "[cache] failed to put: %v\n", err)
		} else if verbose {
			fmt.Fprintf(os.Stderr, "[cache] put: %s\n", cacheKey)
		}
	}

	return result, nil
}

// analyzeDiffWithSurfaceInternal is the internal implementation without caching
func (o *Orchestrator) analyzeDiffWithSurfaceInternal(diffRef, diffText string, changedFiles []string, maxDepth int, withSurface bool) (*model.ImpactResult, error) {
	o.parseCache.Clear()
	o.resetGoDeep()
	o.resetCallGraph()

	// 1. Diff text and changed files are provided by the caller (single git
	// invocation there, shared with the cache-key computation).

	// 2. Parse diff to detect changes
	fileDiffs, err := diff.ParseUnifiedDiff(diffText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	// Start loading surface-diff contracts (git worktree checkout + contract
	// extraction) in the background; addSurfaceDiff joins it at the end of the
	// run, so the I/O-heavy load overlaps the whole analysis pipeline.
	if withSurface {
		o.startSurfaceContractLoad(diffRef, changedFiles)
	}

	// 3. Build reverse import index for scope expansion (#19)
	if err := o.buildReverseImportIndex(); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] Warning: failed to build reverse import index: %v\n", err)
	}

	// 4. Pre-build a call graph so SymbolDiffer can resolve out-of-function var/const references.
	if err := o.buildCallGraph(changedFiles); err != nil {
		return nil, fmt.Errorf("failed to pre-build call graph: %w", err)
	}
	o.callGraph.Finalize()

	differ := diff.NewSymbolDiffer(o.repoPath, diffRef, o.callGraph)
	differ.SetParseCache(o.parseCache)
	allChanges, err := differ.Diff(fileDiffs)
	if err != nil {
		return nil, fmt.Errorf("symbol diff failed: %w", err)
	}

	// 5. Build call graph with expanded scope for better caller detection
	indexFiles, err := o.expandIndexScope(changedFiles)
	if err != nil {
		// Fallback to just changed files if expansion fails
		indexFiles = changedFiles
	}

	if os.Getenv("IMPACT_VERBOSE") != "" {
		siblingCount := len(indexFiles) - len(changedFiles)
		if siblingCount < 0 {
			siblingCount = 0
		}
		fmt.Fprintf(os.Stderr, "[recall] scope expansion: %d files (changed=%d, expanded=%d)\n",
			len(indexFiles), len(changedFiles), siblingCount)
	}

	o.resetCallGraph()
	if err := o.buildCallGraph(indexFiles); err != nil {
		return nil, fmt.Errorf("failed to build call graph: %w", err)
	}

	// Also index test files for test mapping (best-effort)
	testFiles, _ := o.listTestFiles()
	if len(testFiles) > 0 {
		_ = o.buildCallGraph(testFiles)
	}

	// R4: Supplement indexing - for symbols with zero callers, try to find additional importers
	indexedSet := make(map[string]bool, len(indexFiles)+len(testFiles))
	for _, f := range indexFiles {
		indexedSet[f] = true
	}
	for _, f := range testFiles {
		indexedSet[f] = true
	}
	supplementFiles := o.findSupplementFiles(allChanges, indexedSet)
	if len(supplementFiles) > 0 {
		_ = o.buildCallGraph(supplementFiles)

		if os.Getenv("IMPACT_VERBOSE") != "" {
			fmt.Fprintf(os.Stderr, "[recall] supplement pass: indexed %d additional files\n", len(supplementFiles))
		}
	}

	// Finalize callgraph once, after the last edge-mutation pass: sort + dedupe
	// all edges for deterministic query results
	o.callGraph.Finalize()

	result, err := o.analyzeChanges(allChanges, maxDepth, diffRef, testFiles)
	if err != nil {
		return nil, err
	}

	// Enrich with affected contracts if enabled
	if o.config.Analysis.TraceAffectedContracts {
		traceDepth := o.config.Analysis.TraceMaxDepth
		if traceDepth <= 0 {
			traceDepth = 8 // default
		}
		if err := o.enrichWithAffectedContracts(result, traceDepth); err != nil {
			// Log but don't fail the entire analysis
			fmt.Fprintf(os.Stderr, "[orchestrator] Warning: contract enrichment failed: %v\n", err)
		}
	}

	// Optionally compute surface diff
	if withSurface {
		o.addSurfaceDiff(result, diffRef, changedFiles)
	}

	return result, nil
}

// AnalyzeFiles performs impact analysis for specific files (useful for direct file mode)
func (o *Orchestrator) AnalyzeFiles(files []string, maxDepth int) (*model.ImpactResult, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided for analysis")
	}

	o.parseCache.Clear()
	o.resetGoDeep()
	o.resetCallGraph()
	absFiles := o.normalizeFiles(files)

	if err := o.buildCallGraph(absFiles); err != nil {
		return nil, fmt.Errorf("failed to build call graph: %w", err)
	}

	// Finalize callgraph: sort + dedupe all edges for deterministic query results
	o.callGraph.Finalize()

	var changes []*model.Change
	for _, file := range absFiles {
		fileSymbols, err := o.parseFileSymbols(file)
		if err != nil {
			continue
		}

		for _, sym := range fileSymbols.Symbols {
			changes = append(changes, &model.Change{
				Type:      model.ChangeTypeModified,
				Symbol:    sym,
				File:      o.relativeToRepo(file),
				StartLine: sym.Line,
				EndLine:   sym.Line,
			})
		}
	}

	if len(changes) == 0 {
		return nil, fmt.Errorf("no analyzable symbols found in provided files")
	}

	return o.analyzeChanges(changes, maxDepth, "", nil)
}

// parseFileSymbols is a helper to parse file symbols, memoized per run via parseCache
func (o *Orchestrator) parseFileSymbols(filePath string) (*model.FileSymbols, error) {
	language := lang.DetectLanguage(filePath)
	if language == model.LanguageUnknown {
		return nil, fmt.Errorf("unknown language")
	}

	adapter, err := lang.Get(language)
	if err != nil {
		return nil, err
	}

	return o.parseCache.Parse(adapter, filePath)
}

// extractFileCalls is a helper to extract call sites, memoized per run via parseCache
func (o *Orchestrator) extractFileCalls(filePath string) ([]*model.CallSite, error) {
	language := lang.DetectLanguage(filePath)
	if language == model.LanguageUnknown {
		return nil, fmt.Errorf("unknown language")
	}

	adapter, err := lang.Get(language)
	if err != nil {
		return nil, err
	}

	return o.parseCache.ExtractCalls(adapter, filePath)
}

// buildCallGraph builds call graph for given files
func (o *Orchestrator) buildCallGraph(files []string) error {
	if o.callGraph == nil {
		o.callGraph = o.newCallGraph()
	}

	// Kick off the shared Go SSA session load in the background. It is only
	// consumed by buildGoDeepAnalysis below, so the load overlaps the
	// tree-sitter parse loop instead of starting on first use.
	o.startGoDeepSessionLoad(files)

	// Warm the per-run parse cache with a bounded worker pool so the serial
	// indexing loop below hits warm cache entries instead of parsing one file
	// at a time. Only the serial loop mutates the call graph (in the original
	// file order), so the resulting graph is identical to a fully serial pass.
	o.forEachFileConcurrent(files, func(filePath string) {
		language := lang.DetectLanguage(filePath)
		if language == model.LanguageUnknown {
			return
		}
		adapter, err := lang.Get(language)
		if err != nil {
			return
		}
		_, _ = o.parseCache.Parse(adapter, filePath)
		_, _ = o.parseCache.ExtractCalls(adapter, filePath)
	})

	// Register Go declarations for the whole batch before enriching calls.
	// The cache is already warm, so this does not parse again. A bare call
	// may name a free function in a later sibling file.
	for _, file := range files {
		filePath := o.absolutePath(file)
		if lang.DetectLanguage(filePath) != model.LanguageGo {
			continue
		}
		fileSymbols, err := o.parseFileSymbols(filePath)
		if err != nil {
			continue
		}
		for _, sym := range fileSymbols.Symbols {
			o.callGraph.AddSymbol(sym)
		}
	}

	for _, file := range files {
		filePath := o.absolutePath(file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		language := lang.DetectLanguage(filePath)
		if language == model.LanguageUnknown {
			continue
		}

		adapter, err := lang.Get(language)
		if err != nil {
			continue
		}

		// Parse symbols (memoized per run via parseCache)
		fileSymbols, err := o.parseCache.Parse(adapter, filePath)
		if err != nil {
			continue
		}
		// #17-19: Update reverse import index
		o.reverseImports.AddFileImports(filePath, fileSymbols.Imports)

		if language != model.LanguageGo {
			for _, sym := range fileSymbols.Symbols {
				o.callGraph.AddSymbol(sym)
			}
		}

		// Extract calls (memoized per run via parseCache; the R3 callee
		// enrichment below is idempotent, so sharing the slice across
		// passes is safe)
		calls, err := o.parseCache.ExtractCalls(adapter, filePath)
		if err != nil {
			continue
		}

		// Build call graph edges
		for _, call := range calls {
			// Find caller symbol using innermost containment check
			caller := findEnclosingSymbol(fileSymbols.Symbols, call.Line)
			// Adapters may attach an explicit caller when line containment
			// cannot find one (e.g. Rust `impl Trait for Type` blocks are not
			// symbols, so the inheritance edge carries the implementor type).
			if caller == nil && call.Caller != nil {
				caller = call.Caller
			}

			if caller != nil && call.Callee != nil {
				if call.EdgeKind == model.EdgeKindTypeRef && !o.config.Orphan.TypeRefEdges {
					continue
				}
				// Enrich callee package if missing (R3: tree-sitter drops selector operand)
				if call.Callee.Package == "" && call.Callee.Name != "" {
					if call.Callee.Language == model.LanguageGo && call.Callee.Owner == "" && call.Callee.Kind != model.SymbolKindMethod {
						call.Callee.Package = o.inferGoBareCalleePackage(call.Callee.Name, fileSymbols)
					} else {
						call.Callee.Package = o.inferCalleePackage(call.Callee.Name, fileSymbols)
					}
				}
				o.callGraph.AddEdge(caller, call.Callee, call.EdgeKind)
			}
		}
	}

	// #21: Optional SSA/RTA pass for Go files (if use_deep_analysis is enabled)
	if o.config.Go.UseDeepAnalysis && !o.skipGoDeep {
		if err := o.buildGoDeepAnalysis(files); err != nil {
			// Log warning but don't fail - SSA is best-effort
			fmt.Fprintf(os.Stderr, "[orchestrator] Warning: Go deep analysis failed: %v\n", err)
		}
	}

	return nil
}

// buildGoDeepAnalysis runs SSA/RTA analysis on Go packages (#21)
func (o *Orchestrator) buildGoDeepAnalysis(files []string) error {
	// Group files by directory (package). Directories are normalized to
	// absolute paths so they match the shared session's package index even
	// when the orchestrator was constructed with a relative repo path.
	dirMap := make(map[string]bool)
	for _, file := range files {
		absPath := o.absolutePath(file)
		if lang.DetectLanguage(absPath) != model.LanguageGo {
			continue
		}
		dir := filepath.Dir(absPath)
		if !filepath.IsAbs(dir) {
			if abs, err := filepath.Abs(dir); err == nil {
				dir = abs
			}
		}
		dirMap[dir] = true
	}
	if len(dirMap) == 0 {
		return nil
	}

	adapter, err := lang.Get(model.LanguageGo)
	if err != nil {
		return nil
	}

	// Type assert to DeepAnalyzer
	deepAnalyzer, ok := adapter.(lang.DeepAnalyzer)
	if !ok {
		return nil
	}

	buildTags := o.config.Go.BuildTags

	// Preferred path: one shared SSA program per run covering the whole
	// analysis root, with per-directory RTA on top (roots equivalent to a
	// standalone per-directory load). When the session fails to build, or a
	// directory is not covered by it (nested module, files excluded by build
	// constraints), fall back to the legacy per-directory load for those dirs.
	session, hasSession := o.goDeepSessionForRun()

	dirs := make([]string, 0, len(dirMap))
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	absRepo := o.repoPath
	if !filepath.IsAbs(absRepo) {
		if abs, err := filepath.Abs(o.repoPath); err == nil {
			absRepo = abs
		}
	}
	repoPrefix := absRepo + string(os.PathSeparator)
	keepCallFile := func(file string) bool {
		return strings.HasPrefix(o.absolutePath(file), repoPrefix)
	}
	// Both values affect the predicate for relative call-site filenames.
	filterScope := o.repoPath + "\x00" + repoPrefix

	// Process each package directory
	for _, dir := range dirs {
		// Build package call graph (memoized per run: the changed-files,
		// expanded-scope, test-files and supplement passes all re-analyze the
		// same directories, and each SSA/RTA build is expensive)
		var calls []*model.CallSite
		var err error
		if hasSession && session.CoversDir(dir) {
			calls, err = o.goDeepCache.callSitesForDirFiltered(session, dir, buildTags, nil, filterScope, keepCallFile)
		} else {
			calls, err = o.goDeepCache.buildPackageCallGraph(deepAnalyzer, dir, buildTags, nil)
		}
		if err != nil {
			// Log but continue - SSA may fail for various reasons
			fmt.Fprintf(os.Stderr, "[orchestrator] Warning: SSA failed for %s: %v\n", dir, err)
			continue
		}

		// Add SSA-discovered edges to call graph
		for _, call := range calls {
			if call.Callee == nil {
				continue
			}

			// RTA spans the whole dependency graph; callers outside the repo
			// can never resolve to repo symbols. Skip them up front so we
			// don't parse stdlib/module-cache files for nothing.
			if !strings.HasPrefix(o.absolutePath(call.File), repoPrefix) {
				continue
			}

			// SSA calls don't have caller attached; resolve the innermost
			// enclosing symbol by file:line. Skip the edge when no enclosing
			// symbol is found - anonymous (empty-name) callers pollute results
			// and break BFS traversal.
			caller := o.resolveEnclosingSymbol(call.File, call.Line)
			if caller == nil {
				continue
			}
			o.callGraph.AddEdge(caller, call.Callee, call.EdgeKind)
		}
	}

	return nil
}

// goDeepSessionForRun returns the per-run shared Go SSA session, building it
// on first use. The outcome (session or failure) is memoized for the rest of
// the run; a failure never aborts the analysis — callers fall back to the
// legacy per-directory deep analysis path. When an async load was kicked off
// at the start of the call-graph build, this joins it instead of loading
// synchronously; the outcome is identical either way.
func (o *Orchestrator) goDeepSessionForRun() (*godeep.Session, bool) {
	if o.goDeepSessionTried {
		return o.goDeepSession, o.goDeepSession != nil
	}
	o.goDeepSessionTried = true

	if o.goDeepSessionCh != nil {
		res := <-o.goDeepSessionCh
		o.goDeepSessionCh = nil
		o.goDeepSession, o.goDeepSessionErr = res.session, res.err
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] Warning: shared Go SSA session unavailable, using per-directory analysis: %v\n", res.err)
			return nil, false
		}
		if os.Getenv("IMPACT_VERBOSE") != "" {
			fmt.Fprintf(os.Stderr, "[ssa] shared session: loaded %d packages under %s\n", res.session.PackageCount(), res.root)
		}
		return res.session, true
	}

	absRoot, err := filepath.Abs(o.repoPath)
	if err == nil {
		o.goDeepSession, err = godeep.LoadSession(absRoot, o.config.Go.BuildTags)
	}
	if err != nil {
		o.goDeepSessionErr = err
		fmt.Fprintf(os.Stderr, "[orchestrator] Warning: shared Go SSA session unavailable, using per-directory analysis: %v\n", err)
		return nil, false
	}
	if os.Getenv("IMPACT_VERBOSE") != "" {
		fmt.Fprintf(os.Stderr, "[ssa] shared session: loaded %d packages under %s\n", o.goDeepSession.PackageCount(), absRoot)
	}
	return o.goDeepSession, true
}

// startGoDeepSessionLoad kicks off the shared Go SSA session load on a
// background goroutine. It is a no-op when deep analysis is disabled, when
// the file set contains no Go files, or when a load is already in flight or
// done for this run. The load goroutine only sends its result into a
// buffered channel — it never touches orchestrator state — so abandoning an
// in-flight load (resetGoDeep, error paths) is leak- and race-free.
func (o *Orchestrator) startGoDeepSessionLoad(files []string) {
	if !o.config.Go.UseDeepAnalysis || o.skipGoDeep || o.goDeepSessionTried || o.goDeepSessionCh != nil {
		return
	}
	hasGo := false
	for _, file := range files {
		if lang.DetectLanguage(file) == model.LanguageGo {
			hasGo = true
			break
		}
	}
	if !hasGo {
		return
	}
	absRoot, err := filepath.Abs(o.repoPath)
	if err != nil {
		return // the synchronous path in goDeepSessionForRun reports the error
	}
	ch := make(chan goDeepSessionResult, 1)
	o.goDeepSessionCh = ch
	buildTags := o.config.Go.BuildTags
	go func() {
		session, err := godeep.LoadSession(absRoot, buildTags)
		ch <- goDeepSessionResult{session: session, root: absRoot, err: err}
	}()
}

// forEachFileConcurrent runs fn over files with a worker pool bounded by
// GOMAXPROCS. fn receives absolute paths and must be safe for concurrent use
// across distinct files; it must not mutate shared pipeline state (adapters
// are stateless and the parse cache is mutex-protected). Errors are swallowed
// — the serial loops re-handle them through the cache, which memoizes
// failures, so behavior is identical to not warming at all.
func (o *Orchestrator) forEachFileConcurrent(files []string, fn func(filePath string)) {
	if len(files) < 2 {
		return
	}
	workers := runtime.GOMAXPROCS(0)
	if workers > len(files) {
		workers = len(files)
	}
	todo := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range todo {
				fn(o.absolutePath(file))
			}
		}()
	}
	for _, file := range files {
		todo <- file
	}
	close(todo)
	wg.Wait()
}

// resetGoDeep drops all per-run Go deep-analysis state: the call-site cache
// and the shared SSA session. Called at the start of each analysis run. An
// abandoned async load is harmless: its goroutine sends into the detached
// buffered channel and exits.
func (o *Orchestrator) resetGoDeep() {
	o.goDeepCache.Clear()
	o.goDeepSession = nil
	o.goDeepSessionErr = nil
	o.goDeepSessionTried = false
	o.goDeepSessionCh = nil
}

// resolveEnclosingSymbol finds the innermost parsed symbol containing
// file:line. Returns nil when the file cannot be parsed or no symbol
// contains the line.
func (o *Orchestrator) resolveEnclosingSymbol(file string, line int) *model.Symbol {
	fileSymbols, err := o.parseFileSymbols(file)
	if err != nil {
		return nil
	}
	return findEnclosingSymbol(fileSymbols.Symbols, line)
}

// findEnclosingSymbol returns the innermost symbol containing the given line,
// preferring smaller (inner) symbols over outer ones.
func findEnclosingSymbol(symbols []*model.Symbol, line int) *model.Symbol {
	var caller *model.Symbol
	bestSpan := int(^uint(0) >> 1) // MaxInt
	for _, sym := range symbols {
		// Check if line is within symbol bounds
		if line < sym.Line {
			continue
		}
		endLine := sym.EndLine
		if endLine == 0 {
			// No end line: fallback to any symbol starting before this line
			if caller == nil {
				caller = sym
			}
			continue
		}
		if line > endLine {
			continue
		}
		// Calculate span - prefer smaller (inner) symbols
		span := endLine - sym.Line
		if span < bestSpan {
			bestSpan = span
			caller = sym
		}
	}
	return caller
}

// BuildRepoCallGraph builds a call graph for all supported files in the repository
func (o *Orchestrator) BuildRepoCallGraph() (*graph.CallGraph, error) {
	return o.buildRepoCallGraph(false)
}

// buildRepoCallGraph builds the repo-wide call graph. skipGoDeep suppresses
// the Go SSA/RTA edge pass for callers (orphan/whylive) that immediately
// build their own analysis.SSASession, avoiding a second full packages.Load +
// SSA build over the same repository.
func (o *Orchestrator) buildRepoCallGraph(skipGoDeep bool) (*graph.CallGraph, error) {
	files, err := o.listSupportedFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list repository files: %w", err)
	}

	o.parseCache.Clear()
	o.resetGoDeep()
	o.resetCallGraph()
	o.skipGoDeep = skipGoDeep
	defer func() { o.skipGoDeep = false }()
	if err := o.buildCallGraph(files); err != nil {
		return nil, err
	}

	// Finalize callgraph: sort + dedupe all edges for deterministic query results
	o.callGraph.Finalize()

	return o.callGraph, nil
}

// AnalyzeOrphans builds a repository-wide call graph and detects symbols that
// are not reachable from known framework, scheduler, or configured entry points.
func (o *Orchestrator) AnalyzeOrphans(opts analysis.OrphanOptions) (*model.OrphanResult, error) {
	// The SSA session below provides authoritative Go reachability; skip the
	// deep edge pass in the graph build so the full packages.Load + SSA build
	// happens once, not twice.
	cg, err := o.buildRepoCallGraph(o.config.Go.UseDeepAnalysis)
	if err != nil {
		return nil, err
	}

	analyzer := analysis.NewOrphanAnalyzer(cg, o.config.Orphan, contract.All())

	// Enable Go SSA reachability if configured
	if o.config.Go.UseDeepAnalysis {
		fmt.Fprintf(os.Stderr, "[orphan] Go SSA reachability: enabled (entries=%d, tags=%v)\n",
			len(o.config.Go.CustomEntryPoints), o.config.Go.BuildTags)

		session := analysis.NewSSASession(
			o.repoPath,
			o.config.Go.CustomEntryPoints,
			o.config.Go.BuildTags,
			o.config.Go.ReflectionAware,
		)
		provider := analysis.NewGoDeepReachabilityWithSession(session)
		analyzer = analyzer.WithGoReachability(provider)
	} else {
		fmt.Fprintf(os.Stderr, "[orphan] Go SSA reachability: disabled\n")
	}

	return analyzer.Analyze(opts), nil
}

// WhyReachable traces why a symbol is considered reachable from entry points.
func (o *Orchestrator) WhyReachable(targetQName string) (*analysis.WhyReachableResult, error) {
	cg, err := o.buildRepoCallGraph(o.config.Go.UseDeepAnalysis)
	if err != nil {
		return nil, err
	}

	analyzer := analysis.NewOrphanAnalyzer(cg, o.config.Orphan, contract.All())

	if o.config.Go.UseDeepAnalysis {
		session := analysis.NewSSASession(
			o.repoPath,
			o.config.Go.CustomEntryPoints,
			o.config.Go.BuildTags,
			o.config.Go.ReflectionAware,
		)
		provider := analysis.NewGoDeepReachabilityWithSession(session)
		analyzer = analyzer.WithGoReachability(provider)
	}

	return analyzer.WhyReachable(targetQName)
}

// shouldExclude checks if a file should be excluded based on exclude patterns
// Supports both glob patterns (using doublestar) and prefix matching
func (o *Orchestrator) shouldExclude(relPath string) bool {
	// Normalize path separators for consistent matching
	relPath = filepath.ToSlash(relPath)

	for _, ex := range o.excludes {
		// Try glob matching first (supports ** wildcards).
		// Literal patterns can only match the exact string, which the
		// prefix check below already covers.
		if ex.isGlob {
			if matched, _ := doublestar.PathMatch(ex.pattern, relPath); matched {
				return true
			}
		}

		// Try prefix matching (for patterns like "vendor/")
		if strings.HasPrefix(relPath, ex.prefix+"/") || relPath == ex.prefix {
			return true
		}
	}

	return false
}

// Close closes the orchestrator and releases resources
func (o *Orchestrator) Close() error {
	return o.indexMgr.Close()
}

// Config returns the orchestrator's configuration (allows CLI overrides)
func (o *Orchestrator) Config() *config.Config {
	return o.config
}

func (o *Orchestrator) analyzeChanges(allChanges []*model.Change, maxDepth int, diffRef string, testFiles []string) (*model.ImpactResult, error) {
	// Create analyzer with optional entry point checker
	var analyzer *analysis.ImpactAnalyzer
	if o.config.Analysis.StopAtEntryPoints {
		// Build composite entry checker from all registered contract extractors
		allExtractors := contract.All()
		entryChecker := analysis.NewCompositeEntryChecker(allExtractors)
		analyzer = analysis.NewImpactAnalyzerWithEntryChecker(o.callGraph, entryChecker)
	} else {
		analyzer = analysis.NewImpactAnalyzer(o.callGraph)
	}

	result, err := analyzer.Analyze(allChanges, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("impact analysis failed: %w", err)
	}

	// Populate interface implementations so the risk classifier's interface
	// hard rules can trigger (best-effort, never fails the analysis)
	o.populateInterfaceImpls(result, diffRef)

	// Set schema version and timestamp
	result.SchemaVersion = "1.0"
	result.GeneratedAt = time.Now().Format(time.RFC3339)

	mapperOpts := []analysis.TestMapperOption{
		// Route the fallback pass through the per-run parse cache instead of
		// fresh adapter.ParseFile calls.
		analysis.WithParseFuncs(o.parseFileSymbols, o.extractFileCalls),
	}
	if testFiles != nil {
		// Reuse the test-file list collected during call-graph construction
		// instead of re-walking the repository in the fallback pass.
		absTestFiles := make([]string, 0, len(testFiles))
		for _, f := range testFiles {
			absTestFiles = append(absTestFiles, o.absolutePath(f))
		}
		mapperOpts = append(mapperOpts, analysis.WithTestFiles(absTestFiles))
	}
	testMapper := analysis.NewTestMapper(o.callGraph, o.repoPath, mapperOpts...)
	affectedTests, err := testMapper.FindAffectedTests(allChanges)
	if err != nil {
		return nil, fmt.Errorf("test mapping failed: %w", err)
	}
	result.AffectedTests = affectedTests

	classifier := analysis.NewRiskClassifierFromConfig(o.config)
	result.BreakingCount = 0
	result.WarningCount = 0
	result.SafeCount = 0
	for _, change := range result.Changes {
		level, score := classifier.ClassifyRisk(change)
		change.RiskLevel = level
		change.RiskScore = score
		switch level {
		case model.RiskLevelBreaking:
			result.BreakingCount++
		case model.RiskLevelWarning:
			result.WarningCount++
		default:
			result.SafeCount++
		}
	}

	return result, nil
}

// interfaceImplFinder is satisfied by both *godeep.Analyzer (standalone
// types-only load) and *godeep.Session (shared per-run SSA session).
type interfaceImplFinder interface {
	FindInterfaceImplementations(ifaceName string) []*model.Symbol
}

// populateInterfaceImpls fills InterfaceImpls for interface changes so the
// risk classifier's interface hard rules can trigger. Go uses a deep
// (go/types) pass over the repository plus a name-based fallback that matches
// current types against the pre-change interface method set (a method
// signature change breaks implementations, so the post-change source no longer
// satisfies the interface and the go/types pass alone would return nothing).
// Java, Python (base classes) and Rust (traits) reuse the inheritance edges
// already indexed in the call graph. Other languages are left empty. All
// failures degrade silently (warning only).
func (o *Orchestrator) populateInterfaceImpls(result *model.ImpactResult, diffRef string) {
	var goFinder interfaceImplFinder
	goFinderReady := false

	for _, impact := range result.Changes {
		if impact.Change == nil || impact.Change.Symbol == nil {
			continue
		}
		ifaceSym := o.interfaceSymbolFor(impact.Change.Symbol)
		if ifaceSym == nil {
			continue
		}

		switch ifaceSym.Language {
		case model.LanguageGo:
			if !goFinderReady {
				goFinder = o.goInterfaceImplFinder()
				goFinderReady = true
			}
			var impls []*model.Symbol
			if goFinder != nil {
				impls = goFinder.FindInterfaceImplementations(ifaceSym.Name)
			}
			impact.InterfaceImpls = mergeImplSymbols(impls, o.findGoImplementorsOfOldInterface(ifaceSym, diffRef))
		case model.LanguageJava, model.LanguagePython, model.LanguageRust:
			// Inheritance edges point implementor -> interface/base class/trait
			// (Java extends/implements, Python superclasses, Rust impl Trait for
			// Type), so the reverse lookup returns the implementors.
			impact.InterfaceImpls = o.findInheritanceImplementors(ifaceSym)
		}
	}
}

// mergeImplSymbols unions two implementation symbol lists, deduplicated by
// qualified name and file.
func mergeImplSymbols(a, b []*model.Symbol) []*model.Symbol {
	if len(a) == 0 {
		return b
	}
	seen := make(map[string]bool, len(a))
	for _, sym := range a {
		seen[sym.QualifiedName()+"@"+sym.File] = true
	}
	for _, sym := range b {
		key := sym.QualifiedName() + "@" + sym.File
		if !seen[key] {
			seen[key] = true
			a = append(a, sym)
		}
	}
	return a
}

// findGoImplementorsOfOldInterface finds named types whose method names cover
// the pre-change method set of ifaceSym. The go/types pass runs on post-change
// source, where an implementation broken by an interface method signature
// change no longer satisfies the interface; matching against the base-ref
// interface definition recovers exactly those implementations. Matching is
// name-based (signatures ignored) at the symbol level, so it needs no extra
// package loading. All failures degrade to nil.
func (o *Orchestrator) findGoImplementorsOfOldInterface(ifaceSym *model.Symbol, diffRef string) []*model.Symbol {
	if diffRef == "" {
		return nil
	}
	content, err := diff.NewOldFileLoader(o.repoPath, diffRef).LoadOldContent(ifaceSym.File)
	if err != nil {
		return nil
	}
	ifaceMethods, err := golang.InterfaceMethodNames(content, ifaceSym.Name)
	if err != nil || len(ifaceMethods) == 0 {
		return nil
	}

	// Group method names by owning named type. Only owners with a declared
	// type symbol in the graph are considered, which filters out receiver
	// hints recorded on call-site callee symbols.
	typeSymbols := make(map[string]*model.Symbol) // "package.Name" -> declared type
	methodSets := make(map[string]map[string]bool)
	for _, sym := range o.callGraph.Symbols() {
		if sym == nil || sym.Language != model.LanguageGo {
			continue
		}
		switch sym.Kind {
		case model.SymbolKindStruct:
			key := sym.Package + "." + sym.Name
			if _, ok := typeSymbols[key]; !ok {
				typeSymbols[key] = sym
			}
		case model.SymbolKindMethod:
			if sym.Owner == "" {
				continue
			}
			key := sym.Package + "." + sym.Owner
			set := methodSets[key]
			if set == nil {
				set = make(map[string]bool)
				methodSets[key] = set
			}
			set[sym.Name] = true
		}
	}

	var impls []*model.Symbol
	for key, methodNames := range methodSets {
		typeSym, ok := typeSymbols[key]
		if !ok {
			continue
		}
		covers := true
		for _, name := range ifaceMethods {
			if !methodNames[name] {
				covers = false
				break
			}
		}
		if covers {
			impls = append(impls, typeSym)
		}
	}
	sort.Slice(impls, func(i, j int) bool {
		return impls[i].QualifiedName() < impls[j].QualifiedName()
	})
	return impls
}

// goInterfaceImplFinder returns an interface-implementation finder for Go.
// When deep analysis already built the per-run shared session this run, its
// packages are reused (it loads the same repo-root "/..." pattern); otherwise
// fall back to the standalone types-only load. Returns nil on any failure
// (best-effort). It deliberately does not trigger a session build here: when
// deep analysis never ran, the types-only load is cheaper.
func (o *Orchestrator) goInterfaceImplFinder() interfaceImplFinder {
	if o.goDeepSession != nil {
		return o.goDeepSession
	}
	return o.loadGoRepoAnalyzer()
}

// findInheritanceImplementors returns the classes/interfaces connected to an
// interface by inheritance edges. Edges are keyed by the callee name as
// extracted and package enrichment is best-effort, so an interface can end
// up in both the qualified and the bare-name bucket depending on file
// processing order; both are queried and merged. For Java, cross-package hits
// from the bare-name bucket are dropped for precision; Python/Rust
// inheritance edges name the base class/trait directly and cross-module
// implementation is the norm, so their hits are kept.
func (o *Orchestrator) findInheritanceImplementors(ifaceSym *model.Symbol) []*model.Symbol {
	seen := make(map[string]bool)
	var result []*model.Symbol
	add := func(callers []*model.Symbol) {
		for _, c := range callers {
			if c == nil {
				continue
			}
			if ifaceSym.Language == model.LanguageJava && ifaceSym.Package != "" && c.Package != "" && c.Package != ifaceSym.Package && c.File != ifaceSym.File {
				continue
			}
			key := c.QualifiedName() + "@" + c.File
			if !seen[key] {
				seen[key] = true
				result = append(result, c)
			}
		}
	}
	add(o.callGraph.GetCallersByKind(ifaceSym, model.EdgeKindInheritance))
	if ifaceSym.Package != "" {
		bare := &model.Symbol{Name: ifaceSym.Name, Kind: ifaceSym.Kind, Language: ifaceSym.Language}
		add(o.callGraph.GetCallersByKind(bare, model.EdgeKindInheritance))
	}
	return result
}

// interfaceSymbolFor resolves the interface symbol backing a change: the
// changed symbol itself (Go type declarations, Java type add/delete, Python
// base classes, Rust traits), or the interface enclosing a changed method
// (Java emits method-level diffs for interface method changes). Returns nil
// when the change is unrelated to any interface.
func (o *Orchestrator) interfaceSymbolFor(sym *model.Symbol) *model.Symbol {
	if sym.Kind == model.SymbolKindInterface {
		return sym
	}
	// Python base classes play the interface role for their subclasses, which
	// the adapter records as inheritance edges onto the class symbol.
	if sym.Language == model.LanguagePython && sym.Kind == model.SymbolKindClass {
		return sym
	}
	// Method-level change: locate the enclosing interface. Prefer Owner when
	// available; otherwise fall back to file+line containment in the parsed
	// change file (the Java adapter does not set Owner on interface methods,
	// and the graph's interface symbol may be a position-less placeholder).
	if sym.Owner != "" {
		for _, cand := range o.callGraph.GetSymbolsByName(sym.Owner) {
			if !isInterfaceLike(cand, sym.Language) || cand.Package != sym.Package {
				continue
			}
			return cand
		}
	}
	fileSymbols, err := o.parseFileSymbols(o.absolutePath(sym.File))
	if err != nil {
		return nil
	}
	for _, cand := range fileSymbols.Symbols {
		if !isInterfaceLike(cand, sym.Language) {
			continue
		}
		if cand.EndLine > 0 && sym.Line >= cand.Line && sym.Line <= cand.EndLine {
			return cand
		}
	}
	return nil
}

// isInterfaceLike reports whether cand can back interface-implementation
// enrichment for a change in the given language: interfaces everywhere,
// classes for Python (base classes act as the interface for subclasses).
func isInterfaceLike(cand *model.Symbol, lang model.Language) bool {
	if cand.Kind == model.SymbolKindInterface {
		return true
	}
	return lang == model.LanguagePython && cand.Kind == model.SymbolKindClass
}

// loadGoRepoAnalyzer loads all Go packages in the repository for deep type
// analysis. Returns nil on any failure (best-effort).
func (o *Orchestrator) loadGoRepoAnalyzer() *godeep.Analyzer {
	if _, err := os.Stat(filepath.Join(o.repoPath, "go.mod")); err != nil {
		return nil
	}
	absRoot, err := filepath.Abs(o.repoPath)
	if err != nil {
		return nil
	}
	analyzer := godeep.NewAnalyzer()
	if len(o.config.Go.BuildTags) > 0 {
		analyzer = analyzer.WithBuildTags(o.config.Go.BuildTags)
	}
	if err := analyzer.LoadPackages(absRoot + "/..."); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] Warning: Go interface implementation analysis failed: %v\n", err)
		return nil
	}
	return analyzer
}

func (o *Orchestrator) resetCallGraph() {
	o.callGraph = o.newCallGraph()
}

// newCallGraph creates a call graph wired to the orchestrator config and the
// reverse import index (the latter enables cross-package bare-name caller
// recovery in filterCallersByContext).
func (o *Orchestrator) newCallGraph() *graph.CallGraph {
	cg := graph.NewCallGraph()
	cg.SetPermissiveFallback(o.config.Callgraph.PermissiveFallback) // #15
	cg.SetFileImportsFunc(o.fileImportsForGraph)
	return cg
}

// fileImportsForGraph returns the imports recorded for a file in the reverse
// import index. The index is keyed by absolute path during call-graph builds
// and by repo-relative path during the bulk import scan, so try both forms.
func (o *Orchestrator) fileImportsForGraph(file string) []string {
	if imports := o.reverseImports.GetImports(file); len(imports) > 0 {
		return imports
	}
	return o.reverseImports.GetImports(o.relativeToRepo(file))
}

// buildReverseImportIndex builds the reverse import index by parsing
// import statements from all supported files in the repository.
// This is a lightweight pass that only extracts imports, not full symbols or calls.
// #19: Required for cross-package scope expansion.
func (o *Orchestrator) buildReverseImportIndex() error {
	o.reverseImports.Clear()

	allFiles, err := o.listSupportedFiles()
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	// Warm the per-run import cache concurrently (non-Go adapters pay a full
	// tree-sitter parse per file here); the serial loop below hits warm cache
	// entries, so the index contents are identical to a fully serial pass.
	o.forEachFileConcurrent(allFiles, func(filePath string) {
		language := lang.DetectLanguage(filePath)
		if language == model.LanguageUnknown {
			return
		}
		adapter, err := lang.Get(language)
		if err != nil {
			return
		}
		_, _ = o.parseCache.ResolveImports(adapter, filePath)
	})

	for _, filePath := range allFiles {
		// Skip excluded files
		relPath := o.relativeToRepo(filePath)
		if o.shouldExclude(relPath) {
			continue
		}

		language := lang.DetectLanguage(filePath)
		if language == model.LanguageUnknown {
			continue
		}

		adapter, err := lang.Get(language)
		if err != nil {
			continue
		}

		// Only need to resolve imports, not parse full file (memoized per
		// run; non-Go adapters do a full tree-sitter parse here)
		imports, err := o.parseCache.ResolveImports(adapter, filePath)
		if err != nil {
			continue
		}

		o.reverseImports.AddFileImports(relPath, imports)
	}

	return nil
}

func (o *Orchestrator) listSupportedFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(o.repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for exclusion checking
		relPath := o.relativeToRepo(path)

		if d.IsDir() {
			// Check if directory should be excluded
			if o.shouldExclude(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be excluded
		if o.shouldExclude(relPath) {
			return nil
		}

		if lang.DetectLanguage(path) != model.LanguageUnknown {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func (o *Orchestrator) absolutePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(o.repoPath, path)
}

func (o *Orchestrator) relativeToRepo(path string) string {
	if rel, err := filepath.Rel(o.repoPath, path); err == nil {
		return rel
	}
	return path
}

// goModulePath returns the Go module path from go.mod, caching the result.
func (o *Orchestrator) goModulePath() string {
	if o.goModPath != "" {
		return o.goModPath
	}
	data, err := os.ReadFile(filepath.Join(o.repoPath, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			o.goModPath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}
	return o.goModPath
}

// deriveImportPath derives the full import path for a file.
// For Go files, returns modulePath + "/" + relative directory.
// For JS/TS/Python files, returns the relative directory path.
func (o *Orchestrator) deriveImportPath(absPath string) string {
	language := lang.DetectLanguage(absPath)
	if language == model.LanguageGo {
		modPath := o.goModulePath()
		if modPath == "" {
			return ""
		}
		relDir, err := filepath.Rel(o.repoPath, filepath.Dir(absPath))
		if err != nil {
			return ""
		}
		if relDir == "." {
			return modPath
		}
		return modPath + "/" + filepath.ToSlash(relDir)
	}
	// JS/TS/Python: relative dir path
	relDir, err := filepath.Rel(o.repoPath, filepath.Dir(absPath))
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relDir)
}

func (o *Orchestrator) normalizeFiles(files []string) []string {
	normalized := make([]string, 0, len(files))
	for _, file := range files {
		if file == "" {
			continue
		}
		normalized = append(normalized, o.absolutePath(file))
	}
	return normalized
}

// expandIndexScope expands the files to index by including:
//  1. All files in the same directories (sibling files for same package/module)
//  2. Files that import the changed packages (via reverse import index) (#19)
//     Uses BFS multi-hop expansion up to ImportExpansionDepth hops.
func (o *Orchestrator) expandIndexScope(changedFiles []string) ([]string, error) {
	dirSet := make(map[string]bool)
	langMap := make(map[string]model.Language) // track language per directory
	fileSet := make(map[string]bool)           // dedupe final results

	// Collect unique directories
	for _, file := range changedFiles {
		absPath := o.absolutePath(file)
		dir := filepath.Dir(absPath)
		dirSet[dir] = true

		// Remember the language for this directory
		if lang.DetectLanguage(absPath) != model.LanguageUnknown {
			langMap[dir] = lang.DetectLanguage(absPath)
		}
	}

	// Walk each directory and collect files of the same language
	var allFiles []string
	for dir := range dirSet {
		targetLang := langMap[dir]
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fullPath := filepath.Join(dir, entry.Name())
			fileLang := lang.DetectLanguage(fullPath)

			// Include file if it's the same language or if we don't have a target language
			if fileLang != model.LanguageUnknown && (targetLang == model.LanguageUnknown || fileLang == targetLang) {
				relPath := o.relativeToRepo(fullPath)
				if !fileSet[relPath] {
					allFiles = append(allFiles, relPath)
					fileSet[relPath] = true
				}
			}
		}
	}

	// #19: Multi-hop BFS expansion via reverse import index
	maxDepth := o.config.Analysis.ImportExpansionDepth
	if maxDepth <= 0 {
		maxDepth = defaultImportExpansionDepth
	}
	if maxDepth > 4 {
		maxDepth = 4
	}
	// Hard limit to prevent excessive fan-out in large repositories
	const maxExpandedFiles = 500

	changedPackages := o.uniquePackagesOf(changedFiles)
	visited := make(map[string]bool)
	queue := changedPackages

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, pkg := range queue {
			if visited[pkg] {
				continue
			}
			visited[pkg] = true
			importers := o.reverseImports.GetImporters(pkg)
			for _, importer := range importers {
				if !fileSet[importer] && len(allFiles) < maxExpandedFiles {
					allFiles = append(allFiles, importer)
					fileSet[importer] = true
				}
				// For next hop: derive importer's package
				if importerPkg := o.deriveImportPath(o.absolutePath(importer)); importerPkg != "" {
					if !visited[importerPkg] {
						nextQueue = append(nextQueue, importerPkg)
					}
				}
			}
		}
		queue = nextQueue
	}

	return allFiles, nil
}

// uniquePackagesOf extracts unique package names from a list of files
// Returns both short package names and full import paths for reverse import matching.
func (o *Orchestrator) uniquePackagesOf(files []string) []string {
	pkgSet := make(map[string]bool)

	for _, file := range files {
		absPath := o.absolutePath(file)
		fileSymbols, err := o.parseFileSymbols(absPath)
		if err != nil || fileSymbols.Package == "" {
			continue
		}

		pkgSet[fileSymbols.Package] = true

		// Also add full import path to match reverse import index keys
		if importPath := o.deriveImportPath(absPath); importPath != "" {
			pkgSet[importPath] = true
		}
	}

	out := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		out = append(out, p)
	}
	return out
}

// inferGoBareCalleePackage resolves an unqualified Go call. Go methods require
// an explicit receiver; a same-named method or imported function cannot make a
// local free-function call ambiguous. The current file's declarations are
// already registered. Repeated index entries are immaterial.
// Import metadata currently omits aliases and dot-import markers. When there
// is no local function, retain the existing unique-import heuristic for dot
// imports, but consider only exported free functions, never methods.
func (o *Orchestrator) inferGoBareCalleePackage(name string, file *model.FileSymbols) string {
	var importedPackage string
	ambiguous := false
	for _, sym := range o.callGraph.GetSymbolsByName(name) {
		if sym == nil || sym.Name != name || sym.Owner != "" || sym.Kind != model.SymbolKindFunction {
			continue
		}
		if sym.Package == file.Package {
			return file.Package
		}
		if token.IsExported(name) && packageVisibleFromFile(sym.Package, file) {
			if importedPackage != "" && importedPackage != sym.Package {
				ambiguous = true
			}
			importedPackage = sym.Package
		}
	}
	if ambiguous {
		return ""
	}
	return importedPackage
}

// inferCalleePackage tries to resolve the package for a callee with missing package info.
// It uses the callgraph's symbolsByName index and the current file's imports to disambiguate.
func (o *Orchestrator) inferCalleePackage(calleeName string, fileSymbols *model.FileSymbols) string {
	candidates := o.callGraph.GetSymbolsByName(calleeName)
	if len(candidates) == 0 {
		return ""
	}

	// If exactly one candidate, use its package - but only when the current
	// file's context supports it (same package, or an import referencing the
	// candidate package). Otherwise a selector-less bare call would be
	// guessed into a cross-package "exact" edge (known false-positive source).
	if len(candidates) == 1 {
		if packageVisibleFromFile(candidates[0].Package, fileSymbols) {
			return candidates[0].Package
		}
		return ""
	}

	// Multiple candidates: filter by imports of the current file
	importSet := make(map[string]bool, len(fileSymbols.Imports))
	for _, imp := range fileSymbols.Imports {
		importSet[imp] = true
		// Also extract the last path element as a potential short package name
		if idx := strings.LastIndex(imp, "/"); idx >= 0 {
			importSet[imp[idx+1:]] = true
		}
	}

	var matched []*model.Symbol
	for _, cand := range candidates {
		if cand.Package == "" {
			continue
		}
		if importSet[cand.Package] {
			matched = append(matched, cand)
		}
	}

	if len(matched) == 1 {
		return matched[0].Package
	}

	return ""
}

// packageVisibleFromFile reports whether the candidate package is plausibly
// visible from the given file: the file belongs to the same package, or one
// of its imports references the package. Import matching is lenient across
// languages: exact match, Go-style full path whose last element is the
// package, or Java-style class import under the package (pkg.Class).
func packageVisibleFromFile(pkg string, fileSymbols *model.FileSymbols) bool {
	if pkg == "" {
		return false
	}
	if fileSymbols.Package == pkg {
		return true
	}
	for _, imp := range fileSymbols.Imports {
		if imp == pkg ||
			strings.HasSuffix(imp, "/"+pkg) ||
			strings.HasPrefix(imp, pkg+".") {
			return true
		}
	}
	return false
}

// findSupplementFiles finds additional files to index for symbols that have zero callers.
// It queries the reverse import index for packages containing those symbols.
func (o *Orchestrator) findSupplementFiles(changes []*model.Change, indexed map[string]bool) []string {
	pkgSet := make(map[string]bool)
	for _, change := range changes {
		if change.Symbol == nil {
			continue
		}
		if len(o.callGraph.GetCallers(change.Symbol)) > 0 {
			continue
		}
		absPath := o.absolutePath(change.File)
		if pkg := o.deriveImportPath(absPath); pkg != "" {
			pkgSet[pkg] = true
		}
	}

	var files []string
	for pkg := range pkgSet {
		for _, importer := range o.reverseImports.GetImporters(pkg) {
			if !indexed[importer] {
				files = append(files, importer)
				indexed[importer] = true
			}
		}
	}
	return files
}

// listTestFiles finds all test files in the repository
func (o *Orchestrator) listTestFiles() ([]string, error) {
	var testFiles []string

	err := filepath.WalkDir(o.repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for exclusion checking
		relPath := o.relativeToRepo(path)

		if d.IsDir() {
			// Check if directory should be excluded
			if o.shouldExclude(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be excluded
		if o.shouldExclude(relPath) {
			return nil
		}

		language := lang.DetectLanguage(path)
		if language == model.LanguageUnknown {
			return nil
		}

		adapter, err := lang.Get(language)
		if err != nil {
			return nil
		}

		if adapter.IsTestFile(path) {
			testFiles = append(testFiles, o.relativeToRepo(path))
		}

		return nil
	})

	return testFiles, err
}
