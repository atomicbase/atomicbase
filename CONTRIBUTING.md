# Contributing to Atomicbase

Thanks for your interest in contributing to Atomicbase! This document outlines the process for contributing to this project.

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+
- pnpm 9+

### Development Setup

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/atomicbase-2.git
   cd atomicbase-2
   ```

2. Install dependencies:
   ```bash
   pnpm install
   ```

3. Build all packages:
   ```bash
   pnpm build
   ```

4. Start the API server:
   ```bash
   cd api
   make build && make run
   ```

## Development Workflow

### API (Go)

```bash
cd api
```

Build
```bash
make build
```
or
```bash
CGO_ENABLED=1 go build -tags fts5 -o bin/atomicbase
```

Run tests
```bash
make test
```
or
```bash
CGO_ENABLED=1 go test -tags fts5 -v ./...
```

### SDK/Schema/CLI (TypeScript)

```bash
cd packages/sdk  # or schema, cli

# Build
pnpm build

# Watch mode
pnpm dev
```

## Submitting Changes

### Issues

- Search existing issues before creating a new one
- Use a clear, descriptive title
- Include steps to reproduce for bugs
- Include your environment details (OS, Go version, Node version)

### Pull Requests

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature
   ```

2. Make your changes with clear, focused commits

3. Ensure tests pass:
   ```bash
   # API
   cd api && make test

   # Packages
   pnpm build
   ```

4. Push and open a PR against `main`

5. Fill out the PR template with:
   - Summary of changes
   - Related issue (if any)
   - Test plan

### PR Guidelines

- **Size limit**: Keep PRs under 600 lines of changes. Smaller PRs are easier to review and less likely to introduce bugs.
- **Large changes**: Split large features into incremental PRs that can be reviewed and merged independently.
- **AI assistance**: If using AI tools to help write code, you must review and understand all generated code. Be prepared to explain any part of your contribution during review.

### Commit Messages

Write clear commit messages that explain the "why":

```
Add batch insert support for data API

Allows inserting multiple rows in a single request,
reducing round trips for bulk operations.
```

## Code Style

### Go

- Follow standard Go conventions
- Run `go fmt` before committing
- Keep functions focused and small
- Add comments for exported functions

### TypeScript

- Use TypeScript strict mode
- Prefer `const` over `let`
- Use meaningful variable names
- Export types alongside functions

## Testing

- Write tests for new features
- Update tests when modifying existing behavior
- Aim for test coverage on critical paths (auth, data operations)

## Questions?

Open an issue with the "question" label or start a discussion.

## License

By contributing, you agree that your contributions will be licensed under the Apache-2.0 License.
