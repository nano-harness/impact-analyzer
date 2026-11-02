# Contributing to Impact

Thank you for your interest in contributing to Impact! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful and constructive in all interactions.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/nano-harness/impact-analyzer/issues)
2. If not, create a new issue with:
   - Clear title and description
   - Steps to reproduce
   - Expected vs actual behavior
   - Your environment (OS, Go version, etc.)
   - Code samples or test cases if applicable

### Suggesting Features

1. Check [Discussions](https://github.com/nano-harness/impact-analyzer/discussions) for similar ideas
2. Open a new discussion describing:
   - The problem you're trying to solve
   - Your proposed solution
   - Any alternatives you've considered

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Make your changes following our coding standards
4. Write or update tests as needed
5. Update documentation if needed
6. Ensure all tests pass: `make test`
7. Run linters: `make lint`
8. Commit with clear messages
9. Push to your fork
10. Open a Pull Request

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/impact-analyzer.git
cd impact-analyzer

# Install dependencies
make deps

# Build
make build

# Run tests
make test

# Run linters
make lint
```

## Coding Standards

- Follow standard Go conventions
- Use `gofmt` for formatting
- Write clear, descriptive variable and function names
- Add comments for complex logic
- Write tests for new features
- Maintain backward compatibility when possible

## Adding Language Support

To add support for a new language:

1. Create a new adapter in `internal/lang/yourlang/adapter.go`
2. Implement the `LanguageAdapter` interface
3. Add Tree-sitter query files (`*_symbols.scm`, `*_calls.scm`) in the adapter's own directory and embed them there
4. Register the adapter in an `init()` function
5. Add language detection logic to `DetectLanguage()`
6. Update documentation and configuration examples
7. Add test cases

## Project Structure

```
impact-analyzer/
├── cmd/              # CLI commands
├── internal/
│   ├── model/        # Core data models
│   ├── lang/         # Language adapters
│   ├── diff/         # Diff engine
│   ├── graph/        # Graph engine
│   ├── index/        # Persistent index
│   ├── analysis/     # Impact analysis
│   ├── report/       # Output reporters
│   └── orchestrator/ # Workflow coordination
├── skills/           # AI Agent skills
└── testdata/         # Test fixtures
```

## Testing

- Unit tests: Test individual components in isolation
- Integration tests: Test complete workflows
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for >80% code coverage

## Documentation

- Update README.md for user-facing changes
- Update README_zh.md (Chinese documentation)
- Add/update skill documentation for AI integration
- Update CHANGELOG.md
- Add code comments for complex logic

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create and push a version tag: `git tag v0.x.0`
4. GitHub Actions will automatically build and publish the release

## Questions?

Feel free to ask questions in [Discussions](https://github.com/nano-harness/impact-analyzer/discussions).

Thank you for contributing!
