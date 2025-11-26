# Contributing to dbbackup

Thank you for your interest in contributing to dbbackup! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, constructive, and professional in all interactions. We're building enterprise software together.

## How to Contribute

### Reporting Bugs

**Before submitting a bug report:**
- Check existing issues to avoid duplicates
- Verify you're using the latest version
- Collect relevant information (version, OS, database type, error messages)

**Bug Report Template:**
```
**Version:** dbbackup v3.1.0
**OS:** Linux/macOS/BSD
**Database:** PostgreSQL 14 / MySQL 8.0 / MariaDB 10.6
**Command:** The exact command that failed
**Error:** Full error message and stack trace
**Expected:** What you expected to happen
**Actual:** What actually happened
```

### Feature Requests

We welcome feature requests! Please include:
- **Use Case:** Why is this feature needed?
- **Description:** What should the feature do?
- **Examples:** How would it be used?
- **Alternatives:** What workarounds exist today?

### Pull Requests

**Before starting work:**
1. Open an issue to discuss the change
2. Wait for maintainer feedback
3. Fork the repository
4. Create a feature branch

**PR Requirements:**
- ✅ All tests pass (`go test -v ./...`)
- ✅ New tests added for new features
- ✅ Documentation updated (README.md, comments)
- ✅ Code follows project style
- ✅ Commit messages are clear and descriptive
- ✅ No breaking changes without discussion

## Development Setup

### Prerequisites

```bash
# Required
- Go 1.21 or later
- PostgreSQL 9.5+ (for testing)
- MySQL 5.7+ or MariaDB 10.3+ (for testing)
- Docker (optional, for integration tests)

# Install development dependencies
go mod download
```

### Building

```bash
# Build binary
go build -o dbbackup

# Build all platforms
./build_all.sh

# Build Docker image
docker build -t dbbackup:dev .
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run specific test suite
go test -v ./tests/pitr_complete_test.go

# Run with coverage
go test -cover ./...

# Run integration tests (requires databases)
./run_integration_tests.sh
```

### Code Style

**Follow Go best practices:**
- Use `gofmt` for formatting
- Use `go vet` for static analysis
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Write clear, self-documenting code
- Add comments for complex logic

**Project conventions:**
- Package names: lowercase, single word
- Function names: CamelCase, descriptive
- Variables: camelCase, meaningful names
- Constants: UPPER_SNAKE_CASE
- Errors: Wrap with context using `fmt.Errorf`

**Example:**
```go
// Good
func BackupDatabase(ctx context.Context, config *Config) error {
    if err := validateConfig(config); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }
    // ...
}

// Avoid
func backup(c *Config) error {
    // No context, unclear name, no error wrapping
}
```

## Project Structure

```
dbbackup/
├── cmd/                 # CLI commands (Cobra)
├── internal/            # Internal packages
│   ├── backup/         # Backup engine
│   ├── restore/        # Restore engine
│   ├── pitr/           # Point-in-Time Recovery
│   ├── cloud/          # Cloud storage backends
│   ├── crypto/         # Encryption
│   └── config/         # Configuration
├── tests/              # Test suites
├── bin/                # Compiled binaries
├── main.go             # Entry point
└── README.md           # Documentation
```

## Testing Guidelines

**Unit Tests:**
- Test public APIs
- Mock external dependencies
- Use table-driven tests
- Test error cases

**Integration Tests:**
- Test real database operations
- Use Docker containers for isolation
- Clean up resources after tests
- Test all supported database versions

**Example Test:**
```go
func TestBackupRestore(t *testing.T) {
    tests := []struct {
        name     string
        dbType   string
        size     int64
        expected error
    }{
        {"PostgreSQL small", "postgres", 1024, nil},
        {"MySQL large", "mysql", 1024*1024, nil},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Documentation

**Update documentation when:**
- Adding new features
- Changing CLI flags
- Modifying configuration options
- Updating dependencies

**Documentation locations:**
- `README.md` - Main documentation
- `PITR.md` - PITR guide
- `DOCKER.md` - Docker usage
- Code comments - Complex logic
- `CHANGELOG.md` - Version history

## Commit Guidelines

**Commit Message Format:**
```
<type>: <subject>

<body>

<footer>
```

**Types:**
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation only
- `style:` Code style changes (formatting)
- `refactor:` Code refactoring
- `test:` Adding or updating tests
- `chore:` Maintenance tasks

**Examples:**
```
feat: Add Azure Blob Storage backend

Implements Azure Blob Storage backend for cloud backups.
Includes streaming upload/download and metadata preservation.

Closes #42

---

fix: Handle MySQL connection timeout gracefully

Adds retry logic for transient connection failures.
Improves error messages for timeout scenarios.

Fixes #56
```

## Pull Request Process

1. **Create Feature Branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make Changes**
   - Write code
   - Add tests
   - Update documentation

3. **Commit Changes**
   ```bash
   git add -A
   git commit -m "feat: Add my feature"
   ```

4. **Push to Fork**
   ```bash
   git push origin feature/my-feature
   ```

5. **Open Pull Request**
   - Clear title and description
   - Reference related issues
   - Wait for review

6. **Address Feedback**
   - Make requested changes
   - Push updates to same branch
   - Respond to comments

7. **Merge**
   - Maintainer will merge when approved
   - Squash commits if requested

## Release Process (Maintainers)

1. Update version in `main.go`
2. Update `CHANGELOG.md`
3. Create release notes (`RELEASE_NOTES_vX.Y.Z.md`)
4. Commit: `git commit -m "Release vX.Y.Z"`
5. Tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
6. Push: `git push origin main vX.Y.Z`
7. Build binaries: `./build_all.sh`
8. Create GitHub Release with binaries

## Questions?

- **Issues:** https://git.uuxo.net/PlusOne/dbbackup/issues
- **Discussions:** Use issue tracker for now
- **Email:** See SECURITY.md for contact

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

---

**Thank you for contributing to dbbackup!** 🎉
