---
name: impact
description: "Use when reviewing code changes, assessing risk before merge or deploy, identifying affected tests, diffing public API surface, visualizing dependencies, or generating PR/MR impact descriptions with the impact CLI"
---

# Impact - Code Change Impact Assistant

## Overview

Use the `impact` CLI to analyze code change blast radius, identify affected tests, diff public API surface contracts, visualize dependency graphs, and generate structured PR/MR descriptions. Impact supports Go, Java, JavaScript, TypeScript, Rust, and Python projects.

## When to Use

- Before merging a PR/MR to assess what the change affects
- After modifying code to check affected callers and risk levels
- When asked "what does this change affect?" or "is this change safe?"
- When you need a targeted list of existing tests to run
- When creating or updating PR/MR descriptions with impact context
- Before deploying changes that need blast-radius review
- When public contracts such as CLI commands, HTTP routes, RPC methods, or exported symbols may have changed
- When an AI/LLM needs a compact impact context for review or follow-up work

**Not for:** Understanding arbitrary code behavior (use code search) or finding general bugs (use linters/tests).

## Quick Reference

| Goal | Command |
|------|---------|
| Analyze last commit | `impact analyze --diff HEAD~1` |
| Analyze PR branch | `impact analyze --diff origin/main...HEAD` |
| Run deeper analysis | `impact analyze --diff HEAD~1 --depth 10` |
| Analyze dependents of a file | `impact analyze --file path/to/file` |
| Get JSON output | `impact analyze --diff HEAD~1 --format json` |
| Get LLM-friendly output | `impact analyze --diff HEAD~1 --format llm-context --max-tokens 4000` |
| Disable surface diff in analysis | `impact analyze --diff HEAD~1 --no-surface-diff` |
| Annotate callers with coverage | `impact analyze --diff HEAD~1 --use-coverage coverage.out` |
| Disable contract tracing | `impact analyze --diff HEAD~1 --no-trace-contracts` |
| Find affected tests | `impact test --diff HEAD~1` |
| Find orphan code | `impact orphan --min-confidence medium` |
| Print targeted test command | `impact test --diff HEAD~1 --print-command` |
| Annotate affected tests with coverage | `impact test --diff HEAD~1 --use-coverage coverage.out --format json` |
| Diff public API surface | `impact surface --diff HEAD~1` |
| Diff only CLI contracts | `impact surface --diff HEAD~1 --kind cli_command` |
| Use a specific extractor | `impact surface --diff HEAD~1 --extractor cobra` |
| Visualize dependency graph | `impact graph --file path/to/file.go --format mermaid` |
| Show reverse dependencies | `impact graph --file path/to/file.go --reverse` |
| Show callers of a symbol | `impact graph --symbol "pkg.FunctionName"` |
| Generate PR/MR description | `impact cr --diff origin/main...HEAD` |
| Use a custom CR template | `impact cr --diff HEAD~1 --template custom.md` |
| Exclude paths | `impact analyze --diff HEAD~1 --exclude "vendor/**"` |
| Force a language | `impact analyze --diff HEAD~1 --lang python` |
| Use external cache dir | `impact analyze --diff HEAD~1 --index-dir /tmp/impact-cache` |
| Diagnose local setup | `impact doctor` |

## Core Workflow

1. Run `impact analyze --diff <ref>` from the project root.
2. Review results by risk level:
   - **Breaking**: public API or signature changes; review all affected callers.
   - **Warning**: internal changes with callers; verify callers and run relevant tests.
   - **Safe**: low-risk changes such as comments or isolated refactors.
3. Review Surface Contract Changes when present; removed contracts are usually breaking.
4. Run `impact surface --diff <ref>` when you need a focused public API contract diff.
5. Run `impact test --diff <ref>` when you need targeted affected tests.
6. Run `impact graph --file <path>` or `impact graph --symbol <name>` when you need a visual dependency/caller graph.
7. Run `impact cr --diff <ref>` to generate Markdown for the PR/MR description.
8. Add manual business context, migration notes, or rollout details when needed.

## Output Interpretation

```text
Breaking Changes: N
  - func/method signature change
    callers: X
    interfaces: Y implementations affected

Warnings: N
  - internal logic change
    callers: X

Safe: N
  - comments, formatting, isolated refactors
```

## Affected Test Detection

Use `impact test` when you specifically need to know which existing tests to run:

```bash
impact test --diff HEAD~1
impact test --diff HEAD~1 --format json
impact test --diff HEAD~1 --print-command
```

Notes:
- Results may be broader than strictly necessary; over-approximation is safer.
- `--use-coverage` requires an existing Go coverage file.
- In v0.6+, coverage is annotation mode, not filtering mode: all affected tests remain in the output and receive `coverage_status` such as `covered`, `partial`, or `uncovered`.
- `impact test` does not assess risk level; use `impact analyze` for risk classification.

## Public Surface Contract Diff

Use `impact surface` to detect changes in user-facing contracts such as CLI commands, HTTP routes, RPC methods, gRPC services, and exported symbols:

```bash
impact surface --diff HEAD~1
impact surface --diff origin/main...HEAD --kind cli_command,http_route
impact surface --diff HEAD~1 --extractor cobra
impact surface --diff HEAD~1 --format llm-context --max-tokens 4000
```

Supported extractors:
- `cobra` and `clap`: CLI commands for Go and Rust respectively, including SSA-based cobra extraction when enabled by the analyzer
- `gin` and `echo`: Go HTTP routes
- `axum`: Rust HTTP routes
- `grpc`: gRPC services from `.proto` files
- `express` and `nestjs`: JavaScript/TypeScript HTTP routes
- `react-router`: JavaScript/TypeScript frontend routes
- `commander`: JavaScript/TypeScript CLI commands
- `spring-mvc`: Java Spring MVC routes

Surface changes can also appear inside `impact analyze` and `impact cr` output. Use `--no-surface-diff` on `impact analyze` only when faster analysis is more important than public contract coverage.

## Orphan Code Detection

Use `impact orphan` when a feature may have been implemented but not wired into the main flow. It scans the whole repository by default and reports high/medium/low confidence candidates. Use `--diff <ref>` only to raise confidence for changed symbols; it is not the default scan mode. For dynamic Python or reflection-based entries, add `orphan.extra_entries` in `.impact.yaml`.

```bash
impact orphan
impact orphan --min-confidence high
impact orphan --diff HEAD~5 --format json
impact orphan --diff origin/main...HEAD --only-new-islands  # CI gate: only new orphans in changed code
impact orphan --why-reachable "pkg.Symbol"                  # debug why a symbol is not an orphan
```

## Graph Visualization

Use `impact graph` when you need to inspect caller or dependency relationships:

```bash
impact graph --symbol "pkg.FunctionName"
impact graph --symbol "pkg.FunctionName" --depth 3 --format mermaid
impact graph --file path/to/file.go
impact graph --file path/to/file.go --reverse
```

Notes:
- `--symbol` shows callers of a function or method.
- `--file` shows dependencies for a file.
- `--reverse` shows what depends on the file.
- `--format mermaid` is useful for PR comments, design docs, and AI-visible diagrams.

## LLM Context Output

Use `--format llm-context` when another AI tool needs compact, prioritized context:

```bash
impact analyze --diff HEAD~1 --format llm-context --max-tokens 4000
impact surface --diff HEAD~1 --format llm-context --max-tokens 4000
```

This format prioritizes breaking changes and warnings, applies token limits, and summarizes lower-risk details when needed.

## PR/MR Description Generation

Use `impact cr` to generate structured Markdown:

```bash
impact cr --diff origin/main...HEAD > /tmp/impact.md
```

The generated description can include:
- Risk summary with Breaking/Warning/Safe counts
- Breaking change details and affected callers
- Recommended affected tests
- Surface contract additions, removals, and modifications
- Dependency graph diagrams when applicable

## Custom CR Templates

Create `.impact/cr-template.md` using Go Template syntax:

```text
## Impact Analysis

{{range .Changes}}
### {{.RiskLevel}}: {{.Symbol.Name}}
- **File**: {{.Symbol.File}}:{{.Symbol.Line}}
- **Callers**: {{.Reach}}
{{end}}

## Recommended Tests
{{range .Tests}}
- {{.Name}} ({{.Language}})
{{end}}
```

Then run:

```bash
impact cr --diff HEAD~1 --template .impact/cr-template.md
```

## Configuration

Create `.impact.yaml` in the project root for exclusions and depth control:

```yaml
language: go
exclude:
  - "vendor/**"
  - "**/*_test.go"
  - "tools/**"
analyze:
  depth: 5  # or 10 for deeper analysis, -1 for unlimited
  stop_at_entry_points: true

analysis:
  trace_affected_contracts: true
  trace_max_depth: 8
  entry_point_patterns:
    - kind: http_route
    - kind: cli_command
```

Configuration notes:
- Top-level `language`, `exclude`, and `analyze.depth` are the recommended simple settings.
- `analysis.stop_at_entry_points` / `analyze.stop_at_entry_points` stops traversal at framework entry points.
- Contract tracing maps changed methods back to affected public contracts; disable with `--no-trace-contracts` or tune with `--trace-depth`.
- Incremental indexing and analysis caching use `.impact/cache` by default. Override with `--index-dir`, the `INDEX_CACHE_DIR` env var, or `index.cache_dir` in `.impact.yaml` (priority in that order). An external cache dir is useful when the repo must stay clean or when caching is shared across CI jobs.
- `--lang` forces a language (`go`, `java`, `javascript`, `typescript`, `python`, `rust`) when auto-detection picks the wrong one in mixed-language repos.

## CI Integration Examples

```bash
# Analyze changes and filter to Breaking/Warning only
impact analyze --diff origin/main...HEAD --depth 5 --format json > /tmp/analyze.json
jq '.changes[] | select(.risk_level == "warning" or .risk_level == "breaking")' /tmp/analyze.json

# Run targeted tests from JSON output
TESTS=$(impact test --diff origin/main...HEAD --format json | jq -r '.[].file' | sort -u)
go test $TESTS
```

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Running outside project root | `cd` to project root first |
| Treating test results as risk analysis | Use `impact analyze` for risk level |
| Ignoring Warning changes | Warnings with many callers can be high-risk |
| Not checking interface impacts | Breaking interface changes affect implementations |
| Missing API route or CLI changes | Run `impact surface --diff <ref>` or keep surface diff enabled in `impact analyze` |
| Expecting `--use-coverage` to filter tests | Filter JSON client-side by `coverage_status`; v0.6+ annotates instead |
| Sending verbose text output to an LLM | Use `--format llm-context --max-tokens N` |
| Using unlimited depth on large repos | Start with default depth=5 and increase cautiously |
