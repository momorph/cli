# Contributing to MoMorph CLI

Thank you for your interest in contributing to MoMorph CLI! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please be respectful and inclusive in all interactions.

## Getting Started

### Prerequisites

- Go 1.25 or higher
- Git
- Make (optional, for using Makefile targets)
- GoReleaser (for release builds)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/momorph-cli.git
cd momorph-cli
```

3. Add upstream remote:

```bash
git remote add upstream https://github.com/momorph/cli.git
```

## Development Setup

### Installing Dependencies

```bash
go mod download
```

### Building

```bash
# Build binary
go build -o momorph .

# Or use make
make build
```

### Running

```bash
# Run directly
go run main.go <command>

# Or use built binary
./momorph <command>
```

## Development Workflow

### Branching Strategy

- `main` - Production-ready code
- `develop` - Integration branch for features
- `feature/*` - Feature branches
- `fix/*` - Bug fix branches
- `docs/*` - Documentation updates

### Creating a Feature Branch

```bash
# Fetch latest changes
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/your-feature-name
```

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

Examples:
```
feat(auth): add MoMorph token exchange
fix(init): handle empty directory correctly
docs(readme): update installation instructions
test(login): add integration tests for OAuth flow
```

## Code Standards

### Go Style

- Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` and `goimports` for formatting
- Use `golangci-lint` for linting

### Linting

```bash
# Run linter
golangci-lint run

# Or use make
make lint
```

### Error Handling

- Use the `internal/errors` package for CLI errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Provide user-friendly error messages
- Log technical details for debugging

### Logging

- Use the `internal/logger` package
- Use `logger.Debug()` for detailed information
- Use `logger.Info()` for important events
- Never log sensitive data (tokens, passwords)

### File Permissions

- Config files: `0600`
- Config directories: `0700`
- Public files: `0644`
- Public directories: `0755`

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./tests/integration/...

# Run specific test
go test -run TestHelpRootCommand ./tests/integration/...
```

### Writing Tests

- Place unit tests in the same package as the code
- Place integration tests in `tests/integration/`
- Use `testify/assert` for assertions
- Use table-driven tests when appropriate
- Mock external services for unit tests

### Test Structure

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case1", "input1", "expected1"},
        {"case2", "input2", "expected2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := FunctionName(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Pull Request Process

### Before Submitting

1. Ensure all tests pass: `go test ./...`
2. Run linter: `golangci-lint run`
3. Update documentation if needed
4. Add/update tests for new functionality
5. Rebase on latest upstream main

### Submitting

1. Push your branch to your fork
2. Create a Pull Request against `main`
3. Fill in the PR template
4. Request review from maintainers

### PR Requirements

- [ ] Tests pass
- [ ] Linter passes
- [ ] Documentation updated
- [ ] Commit messages follow convention
- [ ] No merge conflicts

### Review Process

- PRs require at least one approval
- Address all review comments
- Keep PRs focused and small when possible
- Squash commits before merging if requested

## Release Process

Releases are automated using GoReleaser and GitHub Actions.

### Version Tagging

```bash
# Create version tag
git tag v1.0.0
git push origin v1.0.0
```

### Release Artifacts

GoReleaser creates:
- Binary releases for Linux, macOS, Windows (amd64, arm64)
- Checksums
- Homebrew formula
- Changelog

### Testing Release Locally

```bash
# Create snapshot release
goreleaser release --snapshot --clean

# Build only
goreleaser build --snapshot --clean
```

## Questions?

If you have questions, please:
1. Check existing issues
2. Search the documentation
3. Open a new issue with the "question" label

Thank you for contributing! 🚀
