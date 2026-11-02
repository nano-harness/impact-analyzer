# Impact - Multi-Language Code Change Impact Analyzer

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)
![Languages](https://img.shields.io/badge/languages-Go%20%7C%20Java%20%7C%20JS%20%7C%20TS%20%7C%20Rust%20%7C%20Python-brightgreen.svg)

[中文文档](./README_zh.md)

Impact is a powerful CLI tool for analyzing code change impact across multi-language projects. It helps you understand the blast radius of your code changes by building call graphs, tracking dependencies, and identifying affected tests.

## 🎯 Features

- **Multi-Language Support**: Analyzes Go, Java, JavaScript, TypeScript, Rust, and Python codebases
- **Dual-Layer Architecture**:
  - Tree-sitter for unified AST parsing across all languages
  - Deep semantic analysis for Go (go/ast + go/types + callgraph)
- **Smart Impact Analysis**: Identifies affected callers, breaking changes, and risk levels
- **Test Mapping**: Automatically finds which tests need to run based on changes
- **Multiple Output Formats**: Terminal text, JSON, Mermaid graphs, and Markdown
- **AI Agent Integration**: Includes Claude Code skill documentation for AI-assisted analysis

### Supported Languages / Frameworks

| Language | Contract extractors |
|---|---|
| Go | cobra, gin, echo, grpc |
| Java | spring-mvc |
| JavaScript / TypeScript | express, commander, NestJS, react-router |
| Rust | axum, clap |
| Python | FastAPI, Flask, Django, Click, Typer, Celery, APScheduler, grpcio, Tornado, aiohttp/Starlette/Sanic |

## 📦 Installation

### Build from Source (recommended)

Requires Go 1.25 or newer.

```bash
git clone https://github.com/nano-harness/impact-analyzer.git
cd impact-analyzer
make build
mkdir -p ~/.local/bin
install -m 755 ./bin/impact ~/.local/bin/impact
impact version
```

Ensure `~/.local/bin` is on your `PATH`.


## 🚀 Quick Start

### Analyze Last Commit

```bash
impact analyze --diff HEAD~1
```

### Analyze Changes vs Main Branch

```bash
impact analyze --diff origin/main...HEAD
```

### Find Affected Tests

```bash
impact test --diff HEAD~1
```

### Generate PR Description

```bash
impact cr --diff origin/main...HEAD
```

## 📖 Usage

### Commands

#### `impact analyze`
Analyze code change impact.

```bash
# Analyze specific diff
impact analyze --diff HEAD~1

# Analyze all dependents of a file
impact analyze --file path/to/file.go

# Limit traversal depth
impact analyze --diff HEAD~1 --depth 5

# Output as JSON
impact analyze --diff HEAD~1 --format json
```

#### `impact test`
Identify affected tests.

```bash
# Find affected tests
impact test --diff HEAD~1

# Use coverage data for precision
impact test --diff HEAD~1 --use-coverage ./coverage.out

# JSON output for CI integration
impact test --diff HEAD~1 --format json
```

#### `impact orphan`
Detect code that is not reachable from known entry points.

For **Go repositories**, orphan detection uses SSA/RTA analysis by default to reduce false positives from interface implementations and indirect calls. Disable with `go.use_deep_analysis: false` in config.

```bash
impact orphan
impact orphan --min-confidence high
impact orphan --diff HEAD~5 --format json
```

Candidates are grouped by high/medium/low confidence. Python results are conservative because dynamic imports and decorators can hide runtime entry points.

#### `impact graph`
Visualize call or dependency graphs.

```bash
# Show callers of a function
impact graph --symbol "pkg.FunctionName"

# Show dependency graph for a file
impact graph --file path/to/file.go

# Output as Mermaid
impact graph --symbol "pkg.Func" --format mermaid
```

#### `impact cr`
Generate code review descriptions.

```bash
# Generate CR description
impact cr --diff origin/main...HEAD

# Use custom template
impact cr --diff HEAD~1 --template ./.impact/cr-template.md
```

## ⚙️ Configuration

Create a `.impact.yaml` file in your project root:

```yaml
# Language-specific settings
go:
  use_deep_analysis: true
  test_patterns:
    - "*_test.go"

java:
  test_patterns:
    - "*Test.java"
    - "*Tests.java"

javascript:
  test_patterns:
    - "*.test.js"
    - "*.spec.js"

typescript:
  test_patterns:
    - "*.test.ts"
    - "*.spec.ts"

# Analysis settings
analysis:
  max_depth: -1  # -1 for unlimited
  include_tests: true
  include_vendor: false

# Risk classification
risk:
  breaking_threshold: 75.0
  warning_threshold: 30.0
```

## 🎨 Output Examples

### Terminal Output

```
Code Change Impact Analysis
==================================================

Summary:
  Total Changes: 3
  ● Breaking: 1
  ● Warning: 1
  ● Safe: 1
  Total Reach: 12 symbols

Breaking Changes:
  ✗ pkg.ProcessData
    File: internal/processor/data.go:45
    Type: modified
    Reach: 8 callers
    Direct Callers:
      - pkg.Handler.ServeHTTP
      - pkg.Worker.Process
```

### JSON Output

```json
{
  "changes": [
    {
      "change": {
        "type": "modified",
        "symbol": {
          "name": "ProcessData",
          "kind": "function",
          "file": "internal/processor/data.go",
          "line": 45
        }
      },
      "risk_level": "breaking",
      "reach": 8,
      "direct_callers": [...]
    }
  ],
  "breaking_count": 1,
  "warning_count": 1,
  "safe_count": 1
}
```

Surface JSON reports use repository-relative `file` paths (for example,
`internal/processor/data.go`). If an absolute path is required, prepend the
repository root in downstream tooling.

## 🤖 AI Agent Integration

Impact includes one consolidated Claude Code skill for AI-assisted impact analysis, affected test detection, and PR/MR impact descriptions.

The `impact` skill is located at [`skills/impact/SKILL.md`](skills/impact/SKILL.md) and follows Claude Code format. It covers impact analysis, affected test detection, and PR/MR impact descriptions.

## 🏗️ Architecture

### Dual-Layer Language Processing

```
┌─────────────────────────────────────────┐
│  Tree-sitter Layer (All Languages)      │
│  - Unified AST parsing                   │
│  - Symbol extraction                     │
│  - Basic call relationship               │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
        ▼                       ▼
┌──────────────┐      ┌──────────────────┐
│ Java/JS/TS/  │      │ Go Deep Analysis │
│ Rust         │      │                  │
│ Syntax-level │      │ go/ast + types   │
│ Analysis     │      │ + callgraph/RTA  │
└──────────────┘      └──────────────────┘
```

### Language Capabilities

| Capability | Go | Java | JS/TS | Rust | Python |
|------------|:--:|:----:|:-----:|:----:|:------:|
| Function/Method Extraction | ✓ | ✓ | ✓ | ✓ | ✓ |
| Call Relationship | Type-precise | Syntax-based | Syntax-based | Syntax-based | Syntax-based |
| Interface Polymorphism | RTA Algorithm | Over-approx | ✗ | ✗ | ✗ |
| Import Resolution | ✓ | ✓ | ✓ | ✗ | ✓ |
| Test File Detection | ✓ | ✓ | ✓ | ✓ | ✓ |

> Note: Rust symbol extraction marks `pub` items as exported for impact analysis,
> but there is not yet a dedicated Rust `exported_symbol` surface contract extractor.

## 🔧 Development

### Prerequisites

- Go 1.25 or higher
- Git

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Running Linters

```bash
make lint
```

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📧 Contact

- Issues: [GitHub Issues](https://github.com/nano-harness/impact-analyzer/issues)
- Discussions: [GitHub Discussions](https://github.com/nano-harness/impact-analyzer/discussions)

## 🙏 Acknowledgments

- [Tree-sitter](https://tree-sitter.github.io/) for unified parsing
- [Cobra](https://github.com/spf13/cobra) for CLI framework
- [go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) for Go bindings
