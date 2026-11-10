package cobra

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// ssaCache is a process-level SSA cache
type ssaCache struct {
	mu      sync.RWMutex
	entries map[string]*ssaCacheEntry // key: package directory abs path
}

// ssaCacheEntry represents a cached SSA program
type ssaCacheEntry struct {
	Prog       *ssa.Program
	PackageDir string
	LoadedAt   time.Time
}

// globalSSACache is the singleton cache instance
var globalSSACache = &ssaCache{
	entries: make(map[string]*ssaCacheEntry),
}

// LoadPackageSSA loads the SSA program for the package containing filePath.
// Returns (prog, package directory, error).
func LoadPackageSSA(filePath string) (*ssa.Program, string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat path: %w", err)
	}

	var absDir string
	if info.IsDir() {
		absDir = absPath
	} else {
		absDir = filepath.Dir(absPath)
	}

	globalSSACache.mu.RLock()
	if entry, ok := globalSSACache.entries[absDir]; ok {
		globalSSACache.mu.RUnlock()
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/pkg_loader] Cache hit for package %s\n", absDir)
		}
		return entry.Prog, absDir, nil
	}
	globalSSACache.mu.RUnlock()

	globalSSACache.mu.Lock()
	defer globalSSACache.mu.Unlock()

	if entry, ok := globalSSACache.entries[absDir]; ok {
		if debugEnabled() {
			fmt.Fprintf(os.Stderr, "[cobra/pkg_loader] Cache hit after lock for package %s\n", absDir)
		}
		return entry.Prog, absDir, nil
	}

	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[cobra/pkg_loader] Loading package-level SSA for %s\n", absDir)
	}

	prog, err := loadPackageWithCfg(absDir)
	if err != nil {
		return nil, "", err
	}

	entry := &ssaCacheEntry{
		Prog:       prog,
		PackageDir: absDir,
		LoadedAt:   time.Now(),
	}
	globalSSACache.entries[absDir] = entry

	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[cobra/pkg_loader] Cached SSA for package %s\n", absDir)
	}

	return entry.Prog, absDir, nil
}

// LoadModuleSSA delegates to LoadPackageSSA for backward compatibility.
// Deprecated: this wrapper remains for older callers; new code should use
// LoadPackageSSA to reflect package-level loading semantics.
func LoadModuleSSA(filePath string) (*ssa.Program, string, error) {
	return LoadPackageSSA(filePath)
}

func loadPackageWithCfg(pkgDir string) (prog *ssa.Program, err error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir: pkgDir,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}

	// Tolerate errors in dependencies — a package requiring a newer Go
	// toolchain than the one running this build must not abort SSA for the
	// target package when that package itself loaded fine. Only fail when no
	// root package produced usable syntax, and surface the first real error
	// message (packages.PrintErrors also writes to os.Stderr, which is not
	// safe alongside the rest of the analysis pipeline).
	if !anyPackageUsable(pkgs) {
		if msg := firstPkgError(pkgs); msg != "" {
			return nil, fmt.Errorf("errors loading package %s: %s", pkgDir, msg)
		}
		return nil, fmt.Errorf("errors loading package %s", pkgDir)
	}

	// Recover creation failures on this goroutine. Package builds remain
	// parallel; panics in x/tools workers cannot be caught by this defer.
	defer func() {
		if r := recover(); r != nil {
			prog = nil
			err = fmt.Errorf("SSA construction panicked for %s: %v", pkgDir, r)
		}
	}()
	prog, _ = ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	if prog == nil {
		return nil, fmt.Errorf("SSA program is nil")
	}
	prog.Build()
	return prog, nil
}

// anyPackageUsable reports whether at least one root package is well-typed —
// the same criterion ssautil uses to decide which packages get SSA code —
// meaning SSA construction has something to build.
func anyPackageUsable(pkgs []*packages.Package) bool {
	for _, pkg := range pkgs {
		if pkg.Types != nil && !pkg.IllTyped {
			return true
		}
	}
	return false
}

// firstPkgError returns the first package or module error message across the
// import graph rooted at pkgs, for diagnostics when loading fails outright.
func firstPkgError(pkgs []*packages.Package) string {
	for pkg := range packages.Postorder(pkgs) {
		if len(pkg.Errors) > 0 {
			return pkg.Errors[0].Msg
		}
		if mod := pkg.Module; mod != nil && mod.Error != nil {
			return mod.Error.Err
		}
	}
	return ""
}

// ResetSSACache clears the cache (for testing only)
func ResetSSACache() {
	globalSSACache.mu.Lock()
	defer globalSSACache.mu.Unlock()

	globalSSACache.entries = make(map[string]*ssaCacheEntry)

	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[cobra/pkg_loader] Cache reset\n")
	}
}
