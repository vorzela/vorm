# Contributing to VORM

We love your input! We want to make contributing to VORM as easy and transparent as possible, whether it's:

- Reporting a bug
- Discussing the current state of the code
- Submitting a fix
- Proposing new features
- Becoming a maintainer

## Development Process

We use GitHub to host code, track issues and feature requests, and accept pull requests.

### Pull Requests

Pull requests are the best way to propose changes to the codebase. We actively welcome your pull requests:

1. **Fork the repo** and create your branch from `main`.
2. **Add tests** if you've added code that should be tested.
3. **Ensure the test suite passes**.
4. **Make sure your code follows the style guidelines**.
5. **Update documentation** if needed.
6. **Create that pull request!**

## Code Style

### Go Code Standards

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` to format your code
- Use `golint` and `go vet` to check for issues
- Write meaningful commit messages

### Naming Conventions

#### Database Objects
- **Tables:** Use plural names (`users`, `products`, `order_items`)
- **Pivot Tables:** Use alphabetical ordering (`product_user`, not `user_product`)
- **Migrations:** Use descriptive names (`create_users_table`, `add_index_to_products`)

#### Go Naming
- **Packages:** lowercase, preferably single word
- **Exported Functions:** PascalCase
- **Internal functions:** camelCase
- **Variables:** camelCase
- **Constants:** PascalCase or UPPER_CASE for package-level

### Code Examples

#### Good ✅
```go
// GenerateMigration creates a new migration file with proper naming conventions.
func GenerateMigration(name string) (*MigrationFile, error) {
    if name == "" {
        return nil, errors.NewValidationError("Migration name cannot be empty", "")
    }
    
    // Generate plural table name
    tableName := utils.Pluralize(utils.ToSnakeCase(name))
    
    return &MigrationFile{
        Name:     name,
        Filename: generateFilename(name),
        Content:  generateTemplate(tableName),
    }, nil
}
```

#### Bad ❌
```go
// generate migration
func generate(n string) (*MigrationFile, error) {
    if n == "" {
        return nil, fmt.Errorf("name empty")
    }
    
    return &MigrationFile{n, "", ""}, nil
}
```

## Testing Guidelines

### Test Coverage

- **Unit tests** for all public functions
- **Integration tests** for database operations
- **End-to-end tests** for CLI commands
- Aim for **80%+ test coverage**

### Test Structure

```go
func TestGenerateMigration(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid migration name",
            input:    "create_users_table",
            expected: "users",
            wantErr:  false,
        },
        {
            name:     "empty migration name",
            input:    "",
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := GenerateMigration(tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            assert.NoError(t, err)
            assert.Contains(t, result.Content, tt.expected)
        })
    }
}
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/migration

# Integration tests (requires PostgreSQL)
VORM_DB_NAME=vorm_test go test ./tests/integration
```

## Commit Message Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format
```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types
- **feat:** A new feature
- **fix:** A bug fix
- **docs:** Documentation only changes
- **style:** Changes that do not affect the meaning of the code
- **refactor:** A code change that neither fixes a bug nor adds a feature
- **perf:** A code change that improves performance
- **test:** Adding missing tests or correcting existing tests
- **chore:** Changes to the build process or auxiliary tools

### Examples

#### Good ✅
```
feat(migration): add rollback to specific migration support

- Add --to flag for rollback command
- Update migration tracker to support target migration
- Add validation for target migration existence

Closes #123
```

```
fix(config): handle missing database.yaml file gracefully

Previously the application would crash if database.yaml was missing.
Now it provides a helpful error message directing users to run 'vorm init'.

Fixes #456
```

#### Bad ❌
```
update stuff
```

```
fix bug
```

## Issue Reporting

### Bug Reports

Great bug reports include:

1. **Clear title** describing the issue
2. **Steps to reproduce** the problem
3. **Expected behavior**
4. **Actual behavior**
5. **Environment details** (OS, Go version, PostgreSQL version)
6. **Error messages** or logs
7. **Minimal example** if possible

### Bug Report Template

```markdown
**Describe the bug**
A clear description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Run command '...'
2. See error

**Expected behavior**
What you expected to happen.

**Environment:**
- OS: [e.g. Ubuntu 20.04]
- Go version: [e.g. 1.19.5]
- PostgreSQL version: [e.g. 14.2]
- VORM version: [e.g. v1.0.0]

**Additional context**
Any other context about the problem.
```

### Feature Requests

Great feature requests include:

1. **Clear description** of the proposed feature
2. **Use case** or problem it solves
3. **Examples** of how it would work
4. **Alternatives** you've considered

## Development Setup

### Prerequisites

- Go 1.19 or later
- PostgreSQL 12 or later
- Git

### Setup Steps

1. **Fork and clone:**
   ```bash
   git clone https://github.com/yourusername/vorm.git
   cd vorm
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Set up development database:**
   ```sql
   CREATE DATABASE vorm_dev;
   CREATE DATABASE vorm_test;
   ```

4. **Configure environment:**
   ```bash
   cp config/.env.example .env
   # Edit .env with your database settings
   ```

5. **Build and test:**
   ```bash
   ./scripts/build.sh
   go test ./...
   ```

## Code Review Process

### What We Look For

- **Correctness:** Does the code work as intended?
- **Clarity:** Is the code easy to understand?
- **Performance:** Are there any obvious performance issues?
- **Security:** Are there any security concerns?
- **Testing:** Are there adequate tests?
- **Documentation:** Is the code properly documented?

### Review Checklist

#### For Authors
- [ ] Code follows style guidelines
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] Commit messages follow conventions
- [ ] No debugging code left in
- [ ] Error handling is appropriate

#### For Reviewers
- [ ] Code is correct and handles edge cases
- [ ] Tests are comprehensive
- [ ] Performance considerations are addressed
- [ ] Security implications are considered
- [ ] Documentation is clear and accurate

## Release Process

### Semantic Versioning

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR:** Breaking changes
- **MINOR:** New features (backward compatible)
- **PATCH:** Bug fixes (backward compatible)

### Release Checklist

- [ ] Update CHANGELOG.md
- [ ] Update version in documentation
- [ ] Create release branch
- [ ] Run full test suite
- [ ] Build all platform binaries
- [ ] Create and test release packages
- [ ] Tag release
- [ ] Create GitHub release
- [ ] Update documentation

## Community Guidelines

### Code of Conduct

Be respectful and inclusive. We want everyone to feel welcome regardless of:

- Experience level
- Gender identity and expression
- Sexual orientation
- Disability
- Personal appearance
- Body size
- Race
- Ethnicity
- Age
- Religion
- Nationality

### Communication

- **Be kind and respectful**
- **Give constructive feedback**
- **Help others learn**
- **Ask questions if something is unclear**
- **Celebrate contributions**

## Recognition

Contributors will be recognized in:

- **CHANGELOG.md** for significant contributions
- **Contributors list** in README.md
- **Release notes** for features and fixes

## Getting Help

- **Documentation:** Check [docs/](docs/) directory
- **Issues:** Search existing issues before creating new ones
- **Discussions:** Use GitHub Discussions for questions
- **Discord:** Join our community Discord (link in README)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to VORM! 🚀
