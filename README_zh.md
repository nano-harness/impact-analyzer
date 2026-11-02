# Impact - 多语言代码变更影响分析器

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)
![Languages](https://img.shields.io/badge/languages-Go%20%7C%20Java%20%7C%20JS%20%7C%20TS%20%7C%20Rust%20%7C%20Python-brightgreen.svg)

[English Documentation](./README.md)

Impact 是一款强大的 CLI 工具，用于分析多语言项目中代码变更的影响范围。通过构建调用图、追踪依赖关系和识别受影响的测试，帮助您理解代码变更的影响半径。

## 🎯 特性

- **多语言支持**：分析 Go、Java、JavaScript、TypeScript、Rust 和 Python 代码库
- **双层架构**：
  - Tree-sitter 提供跨语言统一的 AST 解析
  - Go 语言深度语义分析（go/ast + go/types + callgraph）
- **智能影响分析**：识别受影响的调用方、破坏性变更和风险等级
- **测试映射**：根据变更自动查找需要运行的测试
- **多种输出格式**：终端文本、JSON、Mermaid 图表和 Markdown
- **AI Agent 集成**：包含 Claude Code skill 文档，支持 AI 辅助分析

### 支持语言 / 框架

| 语言 | 契约提取器 |
|---|---|
| Go | cobra, gin, echo, grpc |
| Java | spring-mvc |
| JavaScript / TypeScript | express, commander, NestJS, react-router |
| Rust | axum, clap |
| Python | FastAPI, Flask, Django, Click, Typer, Celery, APScheduler, grpcio, Tornado, aiohttp/Starlette/Sanic |

## 📦 安装

### 源码编译（推荐）

需要 Go 1.25 或更高版本。

```bash
git clone https://github.com/nano-harness/impact-analyzer.git
cd impact-analyzer
make build
mkdir -p ~/.local/bin
install -m 755 ./bin/impact ~/.local/bin/impact
impact version
```

请确保 `~/.local/bin` 已加入 `PATH`。


## 🚀 快速开始

### 分析最后一次提交

```bash
impact analyze --diff HEAD~1
```

### 分析相对于主分支的变更

```bash
impact analyze --diff origin/main...HEAD
```

### 查找受影响的测试

```bash
impact test --diff HEAD~1
```

### 生成 PR 描述

```bash
impact cr --diff origin/main...HEAD
```

## 📖 使用方法

### 命令

#### `impact analyze`
分析代码变更影响。

```bash
# 分析特定 diff
impact analyze --diff HEAD~1

# 分析文件的所有依赖方
impact analyze --file path/to/file.go

# 限制遍历深度
impact analyze --diff HEAD~1 --depth 5

# JSON 输出
impact analyze --diff HEAD~1 --format json
```

#### `impact test`
识别受影响的测试。

```bash
# 查找受影响的测试
impact test --diff HEAD~1

# 使用覆盖率数据提高精度
impact test --diff HEAD~1 --use-coverage ./coverage.out

# JSON 输出用于 CI 集成
impact test --diff HEAD~1 --format json
```

#### `impact orphan`
Detect code that is not reachable from known entry points.

```bash
impact orphan
impact orphan --min-confidence high
impact orphan --diff HEAD~5 --format json
```

Candidates are grouped by high/medium/low confidence. Python results are conservative because dynamic imports and decorators can hide runtime entry points.

#### `impact graph`
可视化调用图或依赖图。

```bash
# 显示函数的调用方
impact graph --symbol "pkg.FunctionName"

# 显示文件的依赖图
impact graph --file path/to/file.go

# 输出为 Mermaid 格式
impact graph --symbol "pkg.Func" --format mermaid
```

#### `impact cr`
生成代码审查描述。

```bash
# 生成 CR 描述
impact cr --diff origin/main...HEAD

# 使用自定义模板
impact cr --diff HEAD~1 --template ./.impact/cr-template.md
```

## ⚙️ 配置

在项目根目录创建 `.impact.yaml` 文件：

```yaml
# 语言特定设置
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

# 分析设置
analysis:
  max_depth: -1  # -1 表示无限制
  include_tests: true
  include_vendor: false

# 风险分类
risk:
  breaking_threshold: 75.0
  warning_threshold: 30.0
```

## 🎨 输出示例

### 终端输出

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

### JSON 输出

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

Surface JSON 报告中的 `file` 路径为相对仓库根的相对路径（例如
`internal/processor/data.go`）。如下游需要绝对路径，请自行拼接仓库根目录。

## 🤖 AI Agent 集成

Impact 包含一个整合后的 Claude Code skill，支持 AI 辅助影响分析、受影响测试识别和 PR/MR 影响说明生成。

`impact` Skill 位于 [`skills/impact/SKILL.md`](skills/impact/SKILL.md)，遵循 Claude Code 格式，覆盖影响分析、受影响测试识别和 PR/MR 影响说明。

## 🏗️ 架构

### 双层语言处理

```
┌─────────────────────────────────────────┐
│  Tree-sitter 层（所有语言）              │
│  - 统一 AST 解析                         │
│  - 符号提取                              │
│  - 基础调用关系                          │
└─────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
        ▼                       ▼
┌──────────────┐      ┌──────────────────┐
│ Java/JS/TS/  │      │ Go 深度分析      │
│ Rust         │      │                  │
│ 语法级别分析 │      │ go/ast + types   │
│              │      │ + callgraph/RTA  │
└──────────────┘      └──────────────────┘
```

### 语言能力对比

| 能力 | Go | Java | JS/TS | Rust | Python |
|------|:--:|:----:|:-----:|:----:|:------:|
| 函数/方法提取 | ✓ | ✓ | ✓ | ✓ | ✓ |
| 调用关系 | 类型精确 | 语法级别 | 语法级别 | 语法级别 | 语法级别 |
| 接口多态 | RTA 算法 | 过度近似 | ✗ | ✗ | ✗ |
| 导入解析 | ✓ | ✓ | ✓ | ✗ | ✓ |
| 测试文件识别 | ✓ | ✓ | ✓ | ✓ | ✓ |

> 注意：Rust 符号提取会将 `pub` 项标记为 exported 供影响分析使用，
> 但目前还没有专门的 Rust `exported_symbol` surface 契约提取器。

## 🔧 开发

### 前置要求

- Go 1.25 或更高版本
- Git

### 构建

```bash
make build
```

### 运行测试

```bash
make test
```

### 运行代码检查

```bash
make lint
```

## 📝 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件。

## 🤝 贡献

欢迎贡献！请随时提交 Pull Request。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交变更 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

## 📧 联系方式

- Issues: [GitHub Issues](https://github.com/nano-harness/impact-analyzer/issues)
- Discussions: [GitHub Discussions](https://github.com/nano-harness/impact-analyzer/discussions)

## 🙏 致谢

- [Tree-sitter](https://tree-sitter.github.io/) 提供统一解析
- [Cobra](https://github.com/spf13/cobra) 提供 CLI 框架
- [go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) 提供 Go 绑定
