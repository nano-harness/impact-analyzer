# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2026-09-13

### Added

- **JavaScript/TypeScript: modern symbol coverage** — Arrow-function and function-valued variable
  declarations (`export const handler = async () => {}`, `const App = () => <div/>`) now produce
  symbols with correct export marking; previously the dominant React/Next.js/Express style was
  invisible to analysis.
- **TypeScript: class members and type-level symbols** — Class and interface methods (with owner
  attribution), abstract methods, type aliases, abstract classes, enums, and namespaces are now
  indexed as symbols.
- **JSX component edges** — Rendering `<UserCard/>` (including `<Foo.Bar/>` member expressions)
  now creates a reference edge to the component symbol, for both `.jsx`/`.js` and `.tsx` files.
- **JS/TS re-export awareness** — `export * from './x'` and `export { a } from './x'` now register
  as imports (barrel files feed the reverse-import index), and `export default Name;` marks the
  named symbol exported.
- **`.mts`/`.cts` recognized** as TypeScript; JS/TS test-file detection now covers
  `.test.`/`.spec.` for `.jsx`/`.mjs`/`.cjs`/`.mts`/`.cts`.
- **Python inheritance edges** — `class Foo(Bar)` (plain or `module.Bar` qualified) emits
  inheritance edges, enabling interface-implementation impact analysis for ABC/mixin/Pydantic
  hierarchies (previously Go/Java-only in `populateInterfaceImpls`; Rust trait impls included too).
- **Python module-level bindings** — Top-level call assignments (`app = FastAPI()`,
  `router = APIRouter()`, `Base = declarative_base()`) are now indexed as symbols.
- **Rust coverage** — Enums (with variants), type aliases, consts, and statics are indexed;
  `impl Trait for Type` emits inheritance edges; files containing `#[cfg(test)]` are treated as
  test files.
- **`impact doctor` improvements** — Language detection now probes up to two directory levels
  (covers `src/` layouts); warns when the Go toolchain is missing while `go.use_deep_analysis`
  is enabled; warns when the repo contains no supported source files.

### Fixed

- **Stale analysis cache across depth changes (correctness)** — The analysis cache key now mixes
  in the effective traversal depth and other analysis-affecting parameters (deep-analysis toggle,
  contract-tracing toggle/depth, surface toggle, build tags). Previously a `--depth 5` run could
  return a cached `max_depth: 1` result.
- **Stale analysis cache on dirty worktrees (correctness)** — The cache key now includes a hash of
  the actual diff content, so uncommitted working-tree edits invalidate cached results when
  diffing against a ref.
- **Added/deleted file paths in diff parsing** — `--- /dev/null` / `+++ /dev/null` headers now map
  to empty old/new paths as intended (previously the literal line text leaked in as a file name,
  so add/delete symbol classification never engaged).
- **Go receiver-owner hints** — Go selector calls (`svc.Find()`) now carry the receiver identifier
  as an owner hint, enabling the same same-name method disambiguation other languages already had
  (precision improvement when SSA deep analysis is unavailable).
- **Go method values / function variables** — `f := s.Process` now produces a call edge to
  `Process` (recall gap closed); direct calls are deduplicated by position.
- **Config file handling** — A malformed `.impact.yaml` now prints a named warning to stderr
  instead of being silently ignored; missing config files stay silent.
- **`analysis.max_depth` / `analysis.trace_max_depth` honored** — Config depth values apply unless
  `--depth` / `--trace-depth` are explicitly passed (previously the flag defaults always won).
- **Unified verbosity** — `--verbose` now also activates `IMPACT_VERBOSE`-gated diagnostics;
  per-run `[cache] hit/miss/put` lines are now verbose-gated instead of always printing.
- **Hermetic golden baseline test** — `TestGoldenBaselines` now builds its own git repository in a
  temp dir instead of diffing the parent repo's `HEAD~1` (which broke on every parent commit).
  `make update-golden` regenerates via `UPDATE_GOLDEN=1 go test`.

### Performance

- **Go files parsed once** — Type-reference extraction moved to tree-sitter queries; the duplicate
  `go/parser` AST pass over every Go file is removed.
- **`impact orphan` / `--why-reachable` SSA double load eliminated** — The full-repo
  `packages.Load` + SSA build now happens exactly once per run.
- **Single graph finalization** — `Finalize()` is called once per call graph instead of 3-4 times
  (each call re-sorted/deduped all edges).
- **Single `git diff` invocation** — Changed-file list and hunks are derived from one diff call.
- **TestMapper reuse** — Test mapping reuses the orchestrator's collected test-file list and
  shared parse cache instead of re-walking and re-parsing the repository.

### Removed

- **Dead query directory** — `internal/lang/treesitter/queries/` (stale, unembedded duplicates of
  the live per-adapter `.scm` files) is deleted; adapters embed their own query files.
- **Rust adapter embedded queries** — Unused `rust_symbols.scm`/`rust_calls.scm` embeds removed
  (the Rust adapter uses a manual AST walk).

## [0.5.4] - 2026-09-04

### Fixed

- **Go SSA version compatibility**: deep analysis and cobra contract extraction no longer fail
  outright when the analyzed module targets a newer Go toolchain than expected
  - Upgraded `golang.org/x/tools` v0.44.0 → v0.49.0 to support current Go toolchains
  - Package loading now tolerates errors in parts of the import graph: SSA is built as long as at
    least one root package is well-typed (matching `ssautil`'s own criterion), instead of aborting
    on any package error anywhere in the graph
  - Load failures now surface the first underlying error message (e.g. "package requires newer Go
    version") instead of an opaque "errors loading packages", and no longer write to the process
    stderr via `packages.PrintErrors`
  - `prog.Build()` panics (observed with code type-checked by a different Go toolchain than the
    embedded x/tools) are recovered and degraded to errors, so the shared-session preload
    goroutine can no longer crash the whole process

## [0.5.3] - 2026-08-15

### Fixed

- **Body-only modifications misclassified as breaking**: old-side symbols are parsed from a temp
  copy whose random basename leaked into path-derived packages (Python module names), splitting a
  modify into delete+add and tripping the breaking hard rule. Basenames are now preserved and
  change entries merge on `kind|owner|name`. Single-line functions now compare the normalized
  signature prefix (text before the body opener) instead of blaming the whole declaration line.
  (Python and Go single-line body edits: breaking → warning.)
- **TypeScript same-name cross-module false positives**: bare-name callers whose own file declares
  a call-compatible same-named symbol are no longer attributed to a changed target in another
  module (local-shadow rule; precision 0.50 → 1.00 on the eval scenario).
- **`affected_tests` output** is now sorted and deterministic (map iteration order leaked into it).

### Added

- **Rust verified at eval level**: four new harness scenarios (cross-module call recall, body-only
  risk, same-name cross-module precision, impl-block method call); Rust adapter gaps in module-path
  derivation, impl owner attribution, and macro/qualified-path call extraction fixed. The eval
  harness now covers 5 languages over 17 scenarios at overall precision=1.00, recall=1.00, with
  zero known failures.

### Performance

- The shared Go SSA session load and surface-diff contract loading now overlap the main pipeline;
  per-file parse/call/import cache warming and per-file contract extraction are parallelized with
  bounded worker pools (stdlib only, deterministic output). Self-analysis benchmarks:
  `AnalyzeDiff` ≈ −6…−8%, `BuildCallGraph` ≈ −5…−10%; larger wins on repos with heavier
  tree-sitter/contract phases.
- Removed a process-global `os.Stderr` swap in `godeep.LoadPackages` that raced once package
  loads run concurrently.

## [0.5.2] - 2026-08-14

### Fixed

- **Go Interface Method Signature Changes**: A method signature change inside an interface is now
  classified as a breaking signature change (was: body-only, risk safe). The interface declaration
  body is treated as signature region, and implementations are matched against the pre-change
  interface method set from the base ref, recovering implementors broken by the change.
- **Cross-package Java Callers**: The reverse import index now registers package-prefix keys for
  class-level imports (`com.example.lib.Helper` also indexed under `com.example.lib`), so
  cross-package callers are detected (was: recall 0). Go/TS/JS import key semantics unchanged.
- **Java Method Owner Attribution**: Method symbols now carry their enclosing class as `Owner`,
  so same-named methods in one package no longer share a graph identity (same-package precision
  0.50 → 1.00 on the eval harness).
- **Cross-package Caller Filter**: The owner-based caller filter now keeps cross-package callers
  whose file imports the target package and whose call site carries no contradicting receiver hint.
- **`INDEX_CACHE_DIR` env var** now actually works (it never reached `viper.Unmarshal`); effective
  cache dir priority is `--index-dir` > `INDEX_CACHE_DIR` > `index.cache_dir` config > `.impact/cache`.
- **`impact doctor`** now checks the effective cache directory instead of the hardcoded default.
- **CLI help** lists all six supported languages (python and rust were missing).

### Documentation

- SKILL.md synchronized with the current CLI surface: `nestjs`/`react-router`/`axum`/`clap`
  extractors, `--index-dir`/`INDEX_CACHE_DIR`/`index.cache_dir` precedence, `--lang` example,
  and orphan `--only-new-islands`/`--why-reachable` usage.

## [0.5.1] - 2026-08-12

### Added

- **`--index-dir` global flag** to select an external index/cache directory (useful for CI caches
  and read-only repos).
- **Precision/recall evaluation harness** with 12 cross-language scenarios and KnownFailures
  tracking (`go test -run Eval ./internal/orchestrator/`).

### Performance

- **SSA session sharing**: one SSA program load per analyze run instead of one per package
  directory (`AnalyzeDiff` ~2.6x faster).
- **Contract extractor dedup**: each file is parsed once across same-language extractors.
- **Forward callee resolution** is narrowed by receiver hint, cutting false same-name matches.

## [0.5.0] - 2026-08-11

### Major - Architecture Upgrade

**Public Contract Diff Framework (REQ-001)**
- **New `impact surface` subcommand**: Analyze changes to public API contracts (CLI commands, HTTP routes, RPC methods, etc.)
  - Supports 7 built-in extractors: cobra, gin, echo, grpc, express, commander, spring-mvc
  - Output formats: text, json, llm-context
  - Filter by `--kind` (cli_command, http_route, rpc_method, grpc_service, exported_symbol)
  - Filter by `--extractor` for specific frameworks
- **Contract Extractors**:
  - **cobra**: CLI commands from spf13/cobra (Go)
  - **gin**: HTTP routes from gin-gonic/gin (Go)
  - **echo**: HTTP routes from labstack/echo (Go)
  - **grpc**: gRPC services from .proto files
  - **express**: HTTP routes from Express.js (JavaScript/TypeScript)
  - **commander**: CLI commands from Commander.js (JavaScript/TypeScript)
  - **spring-mvc**: HTTP routes from Spring MVC annotations (Java)
- **Contract Diff Engine**: SHA256-based signature comparison for detecting added/removed/modified contracts

**Entry Point Configuration (REQ-002)**
- Added `stop_at_entry_points` and `entry_point_patterns` to analysis config
- Entry point patterns support contract kind detection (uses extractors as entry point markers)
- Fallback to file_pattern and symbol_pattern glob matching
- Foundation for smart call graph traversal boundaries (implementation deferred to future version)

**LLM-Context Format (REQ-003)**
- **New `--format llm-context` option** for all subcommands (analyze, test, cr, graph, surface)
- Generates LLM-friendly markdown optimized for AI consumption
- Token limiting with `--max-tokens N` (default: 8000)
- Prioritizes breaking changes > warnings > safe changes
- Auto-truncates with smart summarization

**Surface Diff Integration (REQ-002 + REQ-005)**
- `analyze` command now includes surface contract changes by default
  - New `--no-surface-diff` flag to disable (for performance)
  - Surface changes appear in all output formats (text, json, markdown, llm-context)
  - Breaking contracts (removed) automatically flagged in impact results
- Text reporter shows "🔌 Surface Contract Changes" section
- JSON output includes `surface_diff` field in `ImpactResult`

### Added (P0 - Core Features)
- New `internal/contract/` package for contract abstraction
  - Contract registry with extractor management
  - Git-aware contract loading from refs
  - Diff computation with signature hashing
- New `internal/model/contract.go` with Contract, ContractDiff, ContractKind types
- New `internal/report/llm_context.go` for LLM-friendly output
- New `internal/orchestrator/surface.go` for surface diff integration
- Global `--max-tokens` flag in root command

### Changed
- `Makefile` VERSION updated to 0.5.0
- `ImpactResult` model extended with `SurfaceDiff *ContractDiff` field
- `Orchestrator.AnalyzeDiff()` now calls `AnalyzeDiffWithSurface()`
- `analyze` command adds `--no-surface-diff` flag
- Root command format help updated to include llm-context

### Documentation
- Contract extractors follow consistent interface pattern
- Each extractor provides both Extract() and IsEntryPoint() methods
- Surface diff uses simple line-based parsing for JS/Java (no AST dependencies)

### Note on Deferred Features (P2)
The following features from the original plan are deferred to v0.6.0:
- **Incremental caching** (`--cache-dir`): Foundation exists but not activated
- **Coverage annotation** (`--use-coverage` enhancement): Test command keeps existing behavior
- These are lower priority optimizations that don't affect core functionality


## [0.4.3] - 2026-05-27

### Changed

- **SSA/RTA elevated to primary data source for Go orphan detection** (#39): tree-sitter reachability
  is now a fallback/supplement instead of the primary mechanism.

## [0.4.2] - 2026-05-26

### Added

- **Cross-package Import Expansion**: Impact analysis now correctly resolves full Go import paths
  for reverse import lookup. Previously, short package names (e.g., `"analysis"`) were used as keys
  but the reverse index stored full import paths (e.g., `"github.com/owner/repo/internal/analysis"`),
  causing cross-package callers to never be detected.
- **Multi-hop Reverse Import BFS**: Scope expansion now performs BFS up to configurable depth
  (default 2, max 4) to discover transitive importers. Controlled via
  `analysis.import_expansion_depth` in `.impact.yaml`.
- **Callee Package Enrichment**: `buildCallGraph` now infers missing callee package names using
  the file's import list and the callgraph's symbol index, improving cross-package edge precision.
- **Supplement Indexing Pass**: After initial callgraph construction, symbols with zero callers
  trigger a second indexing pass over their reverse importers, catching late-discovered edges.
- **Recall Diagnostics**: When `IMPACT_VERBOSE=1` is set, stderr reports scope expansion stats
  and supplement pass results for debugging recall issues.
- **Recall Integration Tests**: `TestRecall_CrossPackageCallerDetection` and
  `TestRecall_MultiHopCallerDetection` verify cross-package caller discovery.

- **Tri-state Reachability**: Orphan detection now distinguishes between strong (call/inheritance),
  weak (type_ref/decorator), and unreachable symbols. Symbols only reachable via type references
  are flagged as candidates with reduced confidence, catching "declared but never called" patterns.
- **`--only-new-islands` CLI flag**: Diff-aware mode that only reports orphans within changed
  file ranges. Designed for CI gate usage to detect new orphan code in PRs without noise from
  pre-existing issues.
- **Cluster Aggregation**: Orphan results now include `clusters` field grouping candidates by
  package and file prefix, providing a macro view of "entire modules not wired in."
- **`GetCalleesByKind` graph method**: Allows querying forward edges filtered by edge kind.
- **`WeakReachOffset` confidence config**: Tunable offset (default: -2.0) applied to symbols
  only reachable via type reference edges.

### Fixed

- **PosKey Collision (B1)**: SSA reachability now uses `File+Line+Name` to disambiguate
  multiple declarations on the same line. Previously, same-line short declarations could
  cause one symbol's reachability to mask another's.
- **Type Reference False Negatives (B2)**: Reachability BFS now distinguishes edge kinds.
  Symbols reachable only via `type_ref` or `decorator` edges are marked as `WeakReach`
  instead of being considered fully reachable, preventing missed orphan detections.

## [0.4.1] - 2026-05-20

### Fixed

- **Go SSA Orphan Detection**: Integrated SSA/RTA-based reachability analysis into orphan detection for Go
  - Reduces false positives from interface implementations and indirect calls that tree-sitter misses
  - Enabled by default via `go.use_deep_analysis: true` (set to `false` to disable)
  - Supports `go.custom_entry_points` for custom RTA entry functions
  - Supports `go.build_tags` for conditional compilation
  - Falls back gracefully to tree-sitter-only analysis on SSA errors
  - Only affects Go function/method orphan detection; types/constants/variables unchanged

## [0.4.0] - 2026-04-23

### Breaking Changes
- **`analyze --depth` default changed from -1 to 5**: The default traversal depth for analyze command is now 5 instead of unlimited. This prevents caller explosion in large codebases. Use `--depth -1` explicitly if unlimited depth is needed.

### Added (P0 - Critical Features)
- **Go imports resolver**: Implemented proper Go import path resolution for `graph --file --reverse` command
  - New `GoResolver` resolves Go import paths (e.g., `github.com/org/repo/internal/foo`) to local directories
  - Reads module path from `go.mod`
  - Enables accurate reverse dependency tracking for Go projects
  - **This unblocks the parent requirement for dynamic smoke CI pipeline**
- **Simplified configuration format**: Support for top-level `language`, `exclude`, and `analyze.depth` fields
  - `language: go` - Set main project language
  - `exclude: [...]` - Top-level exclude paths (merged with `analysis.exclude_paths`)
  - `analyze.depth: 5` - Simplified depth setting (merged with `analysis.max_depth`)
  - Backward compatible with existing detailed format
  - Example configuration in `.impact.yaml.example`
- **Global exclude patterns with glob support**: Exclude paths now support glob patterns and apply globally
  - Added `github.com/bmatcuk/doublestar/v4` for `**` wildcard support
  - `exclude_paths` in config or `--exclude` CLI flag now filter all file operations
  - Patterns like `vendor/**`, `**/*_test.go` work consistently
  - Three-level priority: CLI flags > config > defaults
- **JSON output for graph reverse mode**: `graph --file --reverse --format json` now outputs structured JSON
  - Schema includes `schema_version`, `target`, `reverse`, `entries`, `total`
  - `entries` array lists cmd entry points that depend on the target file
  - Enables CI script integration for path-to-command mapping

### Added (P1 - Core Improvements)
- **CLI `--exclude` flag**: All analysis commands (`analyze`, `test`, `cr`) now accept `--exclude` flag
  - Specify exclusions directly on command line: `--exclude "vendor/**" --exclude "tools/**"`
  - Merged with config and default exclusions
- **CLI `--depth` flag for test and cr**: Added depth control to `test` and `cr` commands (default: 5)
  - Previously only `analyze` had depth control
  - Consistent interface across all analysis commands
- **Improved buildReverseDependencyMap**: Rewritten to use language-specific resolvers
  - Uses `GoResolver` for Go projects (proper import path resolution)
  - Falls back to existing resolvers for JavaScript/TypeScript/Java
  - Fixes "No dependencies detected" issue for Go internal/ files

### Fixed
- Fixed `Orchestrator` to respect `exclude_paths` in `listSupportedFiles` and `listTestFiles`
  - Previously hardcoded directory exclusions, now uses configurable patterns
  - Applies `shouldExclude` check to both directories and files during repository walk

### Changed
- Updated `.impact.yaml.example` with simplified format examples
- `Makefile` VERSION updated to 0.4.0
- `Orchestrator` constructor signature: Added `NewOrchestratorWithOptions` accepting CLI excludes
- `helpers.go newOrchestrator` signature: Now accepts `cliExcludes []string` parameter

### Documentation
- Enhanced `.impact.yaml.example` with two-section format: simplified (recommended) and detailed (advanced)
- Added inline comments explaining glob patterns and configuration merging

### Added (continued)

### Added

- **REQ-002**: Real `stop_at_entry_points` implementation - Call graph traversal now stops at framework entry points (HTTP handlers, CLI commands, gRPC methods) when enabled via config
- **REQ-004**: Result-level incremental caching - Analysis results are cached based on diff ref, repo HEAD, config hash, and version for faster repeated runs
- **M4**: Fixed `graph --reverse` - Now uses package import path matching instead of directory resolution for more accurate reverse dependency detection
- **M5**: Automatic version injection - `VERSION` in Makefile now uses `git describe --tags --always --dirty` for automatic version detection


### Changed (continued)

### Changed

- **REQ-005**: Coverage annotation replaces filtering - `--use-coverage` now annotates all tests with coverage status instead of filtering them
- Makefile build command now displays version being built

### Fixed (continued)

### Fixed

- **Go Configuration Keys Now Functional**: Previously declared but unused config keys are now active:
  - `go.use_deep_analysis`: Controls SSA/RTA orphan detection (default: `true`)
  - `go.custom_entry_points`: Custom RTA entry points for orphan analysis
  - `go.build_tags`: Build tags passed to `packages.Load` for conditional compilation

### Fixed

- Improved reverse dependency detection in `graph --reverse` command using package-level import matching

**`impact test --use-coverage` behavior change (REQ-005)**

The `--use-coverage` flag behavior has changed from **filtering mode** to **annotation mode**:

- **Before (v0.5.x)**: Only tests covered by the coverage file were included in the output
- **After (v0.6.0)**: All affected tests are included, each annotated with a `coverage_status` field

**Migration Guide:**

If your CI scripts depend on filtering behavior to run only covered tests, update them to filter on the client side:

```bash
# Old behavior (automatic filtering):
impact test --diff HEAD~1 --use-coverage coverage.out --format json

# New behavior (manual filtering required):
impact test --diff HEAD~1 --use-coverage coverage.out --format json | \
  jq '[.[] | select(.coverage_status == "covered" or .coverage_status == "partial")]'
```

**Output Schema Change:**

```json
// Old schema (v0.5.x)
[
  {"name": "TestFoo", "file": "foo_test.go", "line": 10}
]

// New schema (v0.6.0)
[
  {
    "name": "TestFoo",
    "file": "foo_test.go",
    "line": 10,
    "coverage_status": "covered"
  },
  {
    "name": "TestBar",
    "file": "bar_test.go",
    "line": 20,
    "coverage_status": "uncovered"
  }
]
```

**Coverage Status Values:**
- `"covered"`: Symbol is fully covered by tests
- `"uncovered"`: Symbol has no test coverage
- `"partial"`: Symbol is partially covered
- `""` (empty): Coverage data not provided

## [0.2.3] - 2026-04-27

### Added

- **Surface extractors**: Added Rust clap CLI command extraction and React Router/Ice frontend route extraction.
- **Cobra SSA Extractor (Production)**: Production-ready SSA-based cobra command extraction with bidirectional flag-to-method tracing
  - **Part 1 - Core Algorithm**: Pattern A (factory functions) with 3-layer read algorithm
    - Detects cobra command factory functions (e.g., `func newCmdCommentList() *cobra.Command`)
    - Extracts flag definitions from `Flags()` method calls (e.g., `cmd.Flags().StringVar()`)
    - Layer 1: FreeVar direct matching for anonymous functions
    - Layer 2: FieldAddr fallback for struct-captured variables
    - Layer 3: Named function dispatch for global command patterns
  - **Part 2 - Pattern W + Handlers**: Global command variables and downstream function tracking
    - Pattern W: Package-level `var cmd = &cobra.Command{}` detection
    - Binding owner resolution: Traces flag bindings to their command owner through receiver chain
    - Handlers field: Tracks downstream business functions that consume flag values
    - Contract.Signature extended with `handlers` array: `[{flag, callee, arg_index, depth}]`
    - Handler normalization: Deduplication and lexicographic sorting by (flag, callee, arg_index)
    - Signature hash includes handlers: Refactoring flag consumers triggers modification detection
  - **Part 3a - Module-Level SSA Cache**: Process-level caching for ~1150x speedup on repeated extraction
    - `LoadModuleSSA()`: Loads entire Go module's SSA with caching (first load ~6.5s, cached <6ms)
    - Thread-safe cache with `sync.RWMutex` and double-check locking
    - Temp dir fallback: Single-package load for git_loader temporary files (no caching)
    - Import path filtering fix: Corrected Part 1/2 bug comparing filesystem paths to import paths
    - Cross-package visibility: Module-level loading enables orchestrator → handler traces
  - **Part 3b - Test Suite**: Comprehensive unit and integration tests with 6 testdata scenarios
    - Pattern A simple: Basic factory function with anonymous RunE
    - Pattern A closure: Factory returning closure for RunE
    - Pattern W global: Package-level command variable with init() registration
    - Complex bindings: Multiple flag types (string, int, bool, slice, persistent)
    - FreeVar reads: Layer 1 read algorithm testing
    - Dataflow handlers: Downstream function tracing verification
    - Integration tests: Real cmd/ directory as fixture for end-to-end validation
    - Test utilities: Shared testdata SSA loading and assertion helpers
- **Surface Changes Section**: Added dedicated Surface Changes section to markdown CR template
  - Displays contract changes (ContractsAdded, ContractsRemoved, ContractsModified)
  - Uses emoji indicators: ➕ for added, ⚠️ for removed, 🔄 for modified
  - Automatically included in `impact cr` output when contract changes detected

### Changed

- Consolidated Claude Code skill documentation from `impact-analyze`, `impact-test`, and `impact-cr` into a single `impact` skill.
- Updated skill release publishing and documentation links to use `skills/impact/SKILL.md`.

### BREAKING CHANGE

- **Surface JSON file paths are repository-relative**: `Contract.File` in surface JSON output is now relative to the repository root instead of absolute. Migration: prepend the repository root if absolute paths are required.

**Cobra SSA Extractor Enabled by Default (v0.6.0)**

The SSA-based cobra extractor is now production-ready and enabled by default for projects using `EnableSSA(true)`. This provides:
- **Better accuracy**: Detects factory patterns, global commands, and complex flag bindings
- **Handler tracking**: Identifies which business functions consume each flag
- **Performance**: Module-level caching provides 1150x speedup on repeated extractions

**What Changed:**
- SSA extraction now available for cobra files when enabled
- Contract signatures now include `handlers` field tracking downstream functions
- Signature hashes include handler information: Refactoring handler names/params triggers modification detection

**Migration Impact:**
- **Minimal**: AST fallback still works for cases where SSA is disabled
- **Signature hashes may change**: Existing cobra contracts will have new hashes due to handlers field
  - This is a **one-time change**: Hashes will stabilize after first extraction with v0.6.0
  - CR workflows will show all cobra contracts as "modified" on first v0.6.0 run, then stabilize
- **Performance improvement**: Multi-file analysis is significantly faster with module-level caching

**If You Experience Issues:**
- SSA is opt-in via `EnableSSA(true)`: Enable only when needed
- Report issues: https://github.com/nano-harness/impact-analyzer/issues


## [0.2.0] - 2026-04-22

_Note: the "Symbol.Owner" schema update below was developed under the working label "2.0.0" but first shipped in the v0.2.x line; v0.2.1-v0.3.0 have no changelog entries._

### Breaking Changes
- **Symbol.Owner field**: Added `owner` field to track enclosing class/receiver type
- **QualifiedName changes**: Method qualified names now include owner (e.g., `pkg.Class.method`)
- **Cache invalidation**: All caches invalidated due to schema change (SchemaVersion 2.0.0)

### Fixed
- Caller attribution uses innermost enclosing symbol (#3)
- Python: Comments/strings no longer cause false positive calls (#10)
- Java: IsExported respects class modifiers (#8)

### Added
- Go: Generic instantiation and qualified type support (#4, #5)
- Go: Method receiver tracking (#12)
- Java: Constructor extraction and calls (#6, #7)
- Java: Field type-ref edges (#9)
- Java: Enclosing class tracking (#13)
- Python: Enclosing class tracking (#14)
- Golden baseline test infrastructure (#1)

### Migration
Delete `.impact/cache` before upgrading. New `owner` field in Symbol JSON output (may be empty).


### Fixed (P0 - Critical)
- **Fixed critical adapter registration bug**: Language adapters (Go, Java, JavaScript, TypeScript) were not being registered at runtime due to missing blank imports
  - Added `internal/lang/all` package to aggregate all adapter imports
  - Updated `main.go` to import the `all` package, triggering all adapter `init()` functions
  - This fix makes the tool fully functional - previously all commands would fail with "no adapter registered" errors
- Added adapter health checks to all commands (`analyze`, `test`, `cr`, `graph`)
- Added `IsRegistered()` and `Available()` query functions to the language registry
- Added `TestAllAdaptersRegistered` test to prevent future regressions
- Added diagnostic hints when no changes are detected in analyze command

### Added (P1 - Core Improvements)
- Expanded callgraph coverage to include sibling files in same directory/package
- Automatically index test files for improved test mapping
- Improved symbol matching to find enclosing functions for changes outside function definitions
- Added `AffectedLines()` method to Hunk for precise line-level change tracking
- Centralized risk classification in `RiskClassifier` (removed duplicate logic from `ImpactAnalyzer`)
- **Multi-symbol change detection**: DetectChanges now produces one change per symbol instead of one per hunk
  - Added `DetectChangesWithSymbols()` that maps affected diff lines to enclosing symbols
  - Added `findEnclosingSymbol()` helper with EndLine containment checks
  - Orchestrator uses symbol-aware detection when fileSymbols are available
- **TestMapper forward logic rewrite**: Changed from reverse BFS to forward walking for better accuracy
  - NewTestMapper now accepts `repoRoot` parameter for repository-wide test discovery
  - Walks all test files and checks if they call changed symbols (forward logic)
  - Added `findEnclosingTestFunction()` with proper EndLine containment checks
  - Added `listTestFiles()` to discover test files across the repository
  - Significantly improves cross-directory test detection
- **Configuration integration for risk scoring**: RiskClassifier now reads from config
  - Added `NewRiskClassifierFromConfig()` function
  - Thresholds and weights configurable via `config.Risk` section
  - Orchestrator uses config-based classifier for consistent risk assessment

### Added (P2 - UX & Advanced Features)
- **JSON schema versioning**: Added `schema_version` and `generated_at` fields to ImpactResult
  - Current schema version: "1.0"
  - Enables forward compatibility for API consumers
- **Doctor command**: Added `impact doctor` diagnostic command to check system health
  - Verifies language adapter registration
  - Checks git availability
  - Detects repository language
  - Validates configuration
  - Inspects cache directory
- **Enhanced markdown template for code reviews**: Improved CR description output
  - Added template helper functions: `uniqueFiles()`, `generateTestCommands()`
  - Enhanced formatting with emojis (⚠️ Breaking Changes, ⚡ Warnings, 🧪 Affected Tests)
  - Added risk score display for breaking changes and warnings
  - Added "Suggested Test Commands" section with language-specific bash commands
  - Language-specific test command generators: Go (`go test -run`), Java (`mvn test`), JS/TS (`npx jest`)
  - Organized output: Risk Summary, Changed Files, Breaking Changes, Warnings, Affected Tests
- **Test command --print-command flag**: Displays test commands without executing them
- **Graph command improvements**: Fixed dependency graph recursion and visualization
  - Integrated import path resolvers per language (JavaScript/TypeScript, Java)
  - Fixed `graph --file` recursion to properly traverse transitive dependencies (Root Cause #6)
  - Enhanced Mermaid node sanitization to handle special characters (/, ., :, etc.)
  - renderDependencyText now shows full transitive dependency tree
  - renderDependencyMermaid renders all edges with deduplication
  - Automatically detects project root for import resolution

### Breaking Changes
- Commands now exit with error code 1 when no language adapters are available (previously failed silently)
- JSON output now includes `schema_version` and `generated_at` fields (backward compatible for parsers that ignore unknown fields)
- This is a correctness fix and should not affect normal usage

## [0.1.0] - 2026-04-16

### Added
- Initial release of Impact - Multi-language Code Change Impact Analyzer
- Support for Go, Java, JavaScript, and TypeScript
- Dual-layer architecture:
  - Tree-sitter for unified AST parsing across all languages
  - Deep semantic analysis for Go using go/ast + go/types + callgraph/RTA
- Core commands:
  - `impact analyze` - Analyze code change impact with risk classification
  - `impact test` - Identify affected tests based on changes
  - `impact graph` - Visualize call graphs and dependency graphs
  - `impact cr` - Generate code review descriptions
- Multiple output formats: Terminal text, JSON, Mermaid, Markdown
- SQLite-based persistent index for symbols and call relationships
- Risk classification with configurable thresholds
- Test mapping using language-specific conventions
- AI Agent integration with Claude Code skill documentation:
  - impact-analyze skill
  - impact-test skill
  - impact-cr skill
- Configuration via .impact.yaml
- Comprehensive documentation (English and Chinese)
- GitHub Actions workflow for automated releases

### Features by Language

#### Go
- Type-precise call graph analysis using RTA algorithm
- Interface polymorphism resolution
- Export detection via capitalization
- Test file detection (*_test.go)

#### Java
- Syntax-based call relationship extraction
- Public/private modifier detection
- Test file detection (*Test.java, *Tests.java)
- Over-approximation strategy for polymorphism

#### JavaScript
- Function and class extraction
- Dynamic call tracking (marked as unresolved)
- Test file detection (*.test.js, *.spec.js, __tests__)
- Import resolution

#### TypeScript
- Interface and type alias extraction
- Better static analysis than JavaScript
- Test file detection (*.test.ts, *.spec.ts, __tests__)
- Type annotation support

### Architecture
- Language adapter layer with pluggable design
- Diff engine with git integration
- Graph engine with BFS-based reachability analysis
- Persistent SQLite index
- Analysis engine with risk classification
- Reporting layer with multiple output formats
- Orchestration layer for end-to-end workflow

[0.1.0]: https://github.com/nano-harness/impact-analyzer/releases/tag/v0.1.0
