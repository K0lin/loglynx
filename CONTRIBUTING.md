# Contributing to LogLynx

First off, thank you for considering contributing to LogLynx! It's people like you that make LogLynx a great tool for log analytics.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Enhancements](#suggesting-enhancements)
  - [Your First Code Contribution](#your-first-code-contribution)
  - [Pull Requests](#pull-requests)
- [Development Setup](#development-setup)
- [Styleguides](#styleguides)
  - [Git Commit Messages](#git-commit-messages)
  - [Go Styleguide](#go-styleguide)
  - [JavaScript Styleguide](#javascript-styleguide)
  - [Documentation Styleguide](#documentation-styleguide)
- [Project Structure](#project-structure)
- [Testing](#testing)
- [Additional Notes](#additional-notes)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to [conduct email].

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include as many details as possible:

**Great Bug Reports** tend to have:

- A quick summary and/or background
- Steps to reproduce
  - Be specific!
  - Give sample code if you can
  - Include log output
- What you expected would happen
- What actually happens
- Notes (possibly including why you think this might be happening, or stuff you tried that didn't work)

**Use the Bug Report Template:**

```markdown
**Environment:**
- OS: [e.g., Ubuntu 22.04, Windows 11, macOS 14]
- Docker version (if applicable): [e.g., 24.0.7]
- LogLynx version: [e.g., 3.0.0]
- Go version (if building from source): [e.g., 1.21.5]

**Describe the bug:**
A clear and concise description of what the bug is.

**To Reproduce:**
Steps to reproduce the behavior:
1. Configure '...'
2. Run command '...'
3. See error

**Expected behavior:**
A clear and concise description of what you expected to happen.

**Logs:**
```
Paste relevant logs here
```

**Additional context:**
Add any other context about the problem here.
```

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- **Use a clear and descriptive title**
- **Provide a step-by-step description** of the suggested enhancement
- **Provide specific examples** to demonstrate the steps
- **Describe the current behavior** and **explain the behavior you'd like to see**
- **Explain why this enhancement would be useful** to most LogLynx users
- **List any similar features** in other log analytics tools

### Your First Code Contribution

Unsure where to begin? You can start by looking through these issues:

- `good-first-issue` - Issues which should only require a few lines of code
- `help-wanted` - Issues which should be a bit more involved
- `documentation` - Documentation improvements

### Pull Requests

The process described here has several goals:

- Maintain LogLynx's quality
- Fix problems that are important to users
- Engage the community in working toward the best possible LogLynx
- Enable a sustainable system for maintainers to review contributions

**Please follow these steps:**

1. **Fork the repo** and create your branch from `main`
2. **Make your changes** following our styleguides
3. **Add tests** if you've added code that should be tested
4. **Update documentation** if you've changed APIs or added features
5. **Ensure the test suite passes** (`go test ./...`)
6. **Make sure your code builds** (`go build ./cmd/server`)
7. **Issue the pull request!**

**Pull Request Template:**

```markdown
## Description
Brief description of what this PR does.

## Motivation and Context
Why is this change required? What problem does it solve?
Fixes # (issue)

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## How Has This Been Tested?
Describe the tests you ran to verify your changes.

## Checklist
- [ ] My code follows the code style of this project
- [ ] I have updated the documentation accordingly
- [ ] I have added tests to cover my changes
- [ ] All new and existing tests passed
- [ ] My changes generate no new warnings
- [ ] I have checked my code and corrected any misspellings
```

## Development Setup

### Prerequisites

- **Go 1.25+** 
- **Docker & Docker Compose** (optional but recommended)
- **Git**
- **SQLite3** (for manual database inspection)
- **MaxMind GeoLite2 databases** - See README for download instructions

### Setup Instructions

1. **Clone the repository:**
   ```bash
   git clone https://github.com/K0lin/loglynx.git
   cd loglynx
   ```

2. **Install Go dependencies:**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Set up environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Download GeoIP databases:**
   ```bash
   mkdir -p geoip
   # Download GeoLite2 databases (see README)
   ```

5. **Run the application:**
   ```bash
   # Development mode
   go run ./cmd/server/main.go

   # Or build first
   go build -o loglynx ./cmd/server
   ./loglynx
   ```

6. **Access the dashboard:**
   ```
   http://localhost:8080
   ```

## Styleguides

### Git Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for clear and structured commit messages.

**Format:**

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types:**

- `feat:` - A new feature
- `fix:` - A bug fix
- `docs:` - Documentation only changes
- `style:` - Changes that do not affect the meaning of the code (formatting, missing semi-colons, etc.)
- `refactor:` - A code change that neither fixes a bug nor adds a feature
- `perf:` - A code change that improves performance
- `test:` - Adding missing tests or correcting existing tests
- `build:` - Changes that affect the build system or external dependencies
- `ci:` - Changes to CI configuration files and scripts
- `chore:` - Other changes that don't modify src or test files
- `revert:` - Reverts a previous commit

**Rules:**

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Reference issues and pull requests liberally after the first line
- Use lowercase for the description
- Do not end the description with a period

**Examples:**

```
feat: add WebSocket support for real-time log streaming
fix: allow status code 0 for unknown values
docs: update README with Docker installation steps
perf: optimize GeoIP cache with LRU eviction
refactor: restructure parser registry for better maintainability
test: add unit tests for Traefik parser
fix(database): resolve constraint violation for WebSocket schemes
feat(api)!: breaking change in stats endpoint response format
```

**Breaking Changes:**

Add `!` after the type/scope to indicate a breaking change, or include `BREAKING CHANGE:` in the footer:

```
feat!: redesign API response structure

BREAKING CHANGE: The stats API now returns data in a different format.
Clients will need to update their integration code.
```

### Go Styleguide

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` to format your code
- Run `go vet` to catch common issues
- Follow standard Go project layout
- Write meaningful variable and function names
- Add comments for exported functions and types
- Use structured logging (pterm)

**Code Style:**

```go
// Good: Clear function name, documented, error handling
// EnrichWithGeoIP adds geographic information to the HTTP request model
func (e *GeoIPEnricher) EnrichWithGeoIP(req *models.HTTPRequest) error {
    if req.ClientIP == "" {
        return fmt.Errorf("client IP is empty")
    }

    // Check cache first
    if cached, ok := e.getFromCache(req.ClientIP); ok {
        req.GeoCountry = cached.Country
        req.GeoCity = cached.City
        return nil
    }

    // Lookup in database
    record, err := e.lookupIP(req.ClientIP)
    if err != nil {
        return fmt.Errorf("failed to lookup IP: %w", err)
    }

    // Update request
    req.GeoCountry = record.Country
    req.GeoCity = record.City

    // Cache for future use
    e.addToCache(req.ClientIP, record)

    return nil
}
```

### JavaScript Styleguide

- Use ES6+ features (const/let, arrow functions, template literals)
- Use meaningful variable names
- Add JSDoc comments for functions
- Keep functions small and focused
- Use async/await for asynchronous code

**Code Style:**

```javascript
/**
 * Fetch log processing statistics from the API
 * @returns {Promise<Object>} Processing stats with percentage and bytes
 */
async getLogProcessingStats() {
    try {
        const response = await fetch('/api/v1/stats/log-processing');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        return { success: true, data };
    } catch (error) {
        console.error('Failed to fetch processing stats:', error);
        return { success: false, error: error.message };
    }
}
```

### Documentation Styleguide

- Use Markdown for all documentation
- Use code blocks with language specification
- Include examples where applicable
- Keep line length to ~80 characters for readability
- Use proper heading hierarchy

## Project Structure

```
loglynx/
├── cmd/
│   └── server/          # Application entry point
├── internal/            # Private application code
│   ├── api/            # HTTP API handlers
│   ├── config/         # Configuration management
│   ├── database/       # Database layer (GORM, migrations)
│   ├── enrichment/     # GeoIP enrichment
│   ├── ingestion/      # Log file processing
│   └── parser/         # Log format parsers (Traefik, etc.)
├── web/                # Frontend assets
│   ├── static/         # CSS, JS, images
│   └── templates/      # HTML templates
├── scripts/            # Utility scripts and migrations
├── Dockerfile          # Container image definition
├── docker-compose.yml  # Docker Compose configuration
├── go.mod              # Go module dependencies
└── README.md           # Main documentation
```

### Key Components

- **Parser**: Handles log file format parsing (Traefik JSON/Common)
- **Ingestion**: Manages file watching, reading, and batch processing
- **Enrichment**: Adds GeoIP data and user-agent parsing
- **Database**: GORM models, repositories, and migrations
- **API**: RESTful endpoints for dashboard data
- **Frontend**: Vanilla JS with Chart.js for visualization

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test ./internal/parser/traefik
```

### Writing Tests

```go
func TestParseTraefikLog(t *testing.T) {
    parser := NewTraefikParser(logger)

    testCases := []struct {
        name     string
        input    string
        expected *models.HTTPRequest
        wantErr  bool
    }{
        {
            name:  "valid JSON log",
            input: `{"ClientHost":"192.168.1.1","DownstreamStatus":200}`,
            expected: &models.HTTPRequest{
                ClientIP:   "192.168.1.1",
                StatusCode: 200,
            },
            wantErr: false,
        },
        // More test cases...
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := parser.Parse(tc.input)
            if tc.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tc.expected.ClientIP, result.ClientIP)
            assert.Equal(t, tc.expected.StatusCode, result.StatusCode)
        })
    }
}
```

## Additional Notes

### Issue and Pull Request Labels

- `bug` - Something isn't working
- `enhancement` - New feature or request
- `documentation` - Improvements or additions to documentation
- `good-first-issue` - Good for newcomers
- `help-wanted` - Extra attention is needed
- `question` - Further information is requested
- `wontfix` - This will not be worked on
- `duplicate` - This issue or pull request already exists

### Development Tips

1. **Use structured logging:**
   ```go
   logger.Info("Processing log file",
       logger.Args("file", filename, "size", fileSize))
   ```

2. **Handle errors properly:**
   ```go
   if err != nil {
       return fmt.Errorf("failed to process: %w", err)
   }
   ```

3. **Write self-documenting code:**
   - Use clear variable names
   - Add comments for complex logic
   - Keep functions focused on one task

4. **Test edge cases:**
   - Empty inputs
   - Invalid data
   - Concurrent access
   - Resource limits

5. **Performance considerations:**
   - Use connection pooling
   - Batch database operations
   - Cache frequently accessed data
   - Profile before optimizing

### Getting Help

- 💬 **Discussions**: Use GitHub Discussions for questions
- 🐛 **Issues**: Report bugs via [GitHub Issues](https://github.com/K0lin/loglynx/issues)
- 📖 **Documentation**: Check README.md

### Recognition

Contributors will be recognized in:
- GitHub contributors page
- Release notes for significant contributions
- README.md contributors section (planned)

Thank you for contributing to LogLynx! 🎉

---

**Questions?** Feel free to ask in GitHub Discussions or open an issue.
