# envlens

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

**envlens** is a zero-dependency Go CLI tool for comparing, validating, and displaying `.env` files.
It targets developers who want a language-agnostic, single-binary tool that can be dropped into any CI pipeline.

Design priorities:

- Single static binary, stdlib-only at runtime
- Exit codes are the CI-integration contract (0 clean, 1 findings, 2 usage/I/O)
- Secrets are masked by key-name suffix across every command
- Validation rules are inferred from key names (no schema file to maintain)

### Commands

- `diff <file1> <file2>` — Compare keys between two `.env` files; exit code 1 on mismatch, prints `No differences found.` on clean; CI-friendly
- `validate <file>` — Validate values against rules auto-inferred from key name suffixes; only keys matching a suffix rule appear in output, others are silently skipped
- `show <file>` — Display `.env` contents as a formatted table with automatic masking
- `--version` — Print the version string embedded at build time; exit code 0

---

## Architecture

    envlens/
    ├── cmd/envlens/main.go
    ├── internal/
    │   ├── parser/
    │   │   └── testdata/       # fixtures for parser unit tests
    │   ├── diff/
    │   ├── validate/           # table-driven tests; testdata/ only if fixtures are needed
    │   └── display/
    ├── e2e/
    │   ├── e2e_test.go
    │   └── testdata/           # .env fixtures + compiled binary (gitignored)
    ├── .github/
    │   ├── dependabot.yml
    │   ├── ISSUE_TEMPLATE/
    │   │   ├── bug_report.md
    │   │   └── feature_request.md
    │   ├── pull_request_template.md
    │   └── workflows/
    │       ├── ci.yml
    │       ├── commitlint.yml
    │       ├── release.yml
    │       └── release-please.yml
    ├── .gitignore
    ├── .golangci.yaml
    ├── .goreleaser.yaml
    ├── .release-please-manifest.json
    ├── codecov.yml
    ├── CLAUDE.md               # This file
    ├── CONTRIBUTING.md
    ├── LICENSE
    ├── Makefile
    ├── README.md
    ├── release-please-config.json
    ├── go.mod
    └── go.sum

### Key Design Patterns

1. **Zero external dependencies**: Parser, display, and CLI are all stdlib only.
   `github.com/stretchr/testify` is the sole dev dependency (test assertions only).
2. **`io.Writer` throughout**: All output functions accept `io.Writer`; commands never write directly to `os.Stdout`.
   This makes every command trivially testable.
3. **Exit codes as contracts**: `diff` and `validate` return exit code 1 on findings, 0 on clean.
   This is the primary CI integration surface.
4. **Masking by key name suffix**: Masking and validation rules share the same suffix-matching logic
   defined in `internal/validate/rules.go` to avoid duplication. Masked values are always displayed
   as `****` regardless of the command — the format is consistent across `show` and `diff --values`.
5. **Version embedding**: The `version` variable in `cmd/envlens/main.go` defaults to `"dev"` and is
   overwritten at build time by GoReleaser via ldflags (`-X main.version={{.Version}}`).

---

## Parser Specification (v1)

The parser is hand-written using stdlib (`bufio`, `strings`). It intentionally covers a limited subset.

### Supported

    # comment line
    KEY=value
    KEY="double quoted value"
    KEY='single quoted value'
    KEY=                        # empty value (allowed)
    KEY=value  # inline comment
    export KEY=value            # export prefix (stripped during parsing)

### Not Supported in v1 (document in README)

- Variable interpolation (`${OTHER_KEY}`)
- Multi-line values (backslash continuation or newline inside quotes)
- Nested quotes

Encountering unsupported syntax must not cause a panic; the parser should skip the line and optionally warn.

### Semantic nuances worth remembering

- `KEY=#value` — the value is `#value`. `#` only introduces an inline
  comment when preceded by whitespace.
- `KEY=  # trailing` — since there is whitespace before `#`, the value is
  empty and `# trailing` is the comment.
- `KEY="value"trailing` — trailing text after a closing quote is dropped
  silently (documented behavior, no warning).
- Bare IPv6 literals without a scheme (e.g. `[::1]:8080`) are rejected by
  the `_URL`/`_URI` validator; use a scheme-qualified form instead.

---

## Validation Rules

Rules are matched by **key name suffix** (case-insensitive).

| Suffix pattern | Validation |
|---|---|
| `_URL`, `_URI` | Non-empty and whitespace-free. Scheme-qualified values (`scheme://host`) require a scheme; `file://` URIs may have an empty host, other schemes require one. Bare values are accepted when the host portion (before any `:port`) is `localhost` (case-insensitive) or contains `.` (FQDNs, IPv4 literals). |
| `_PORT` | Integer in range 1–65535 |
| `_KEY`, `_SECRET`, `_TOKEN`, `_PASSWORD` | Non-empty |
| `_EMAIL`, `_ADDRESS` | Valid email format |
| `_ENABLED`, `_DEBUG`, `_FLAG` | One of: `true`, `false`, `1`, `0` |

Keys with no matching suffix are skipped silently (unless `--strict` is passed) and do not appear in output.
When `--strict` is passed, unmatched keys appear as `? KEY  no rule defined`.

---

## Development Commands

Use `make` for common tasks:

    make build    # go build -o bin/envlens ./cmd/envlens
    make test     # go test -race ./internal/... ./cmd/...  (unit tests only; avoids running e2e twice in make all)
    make cover    # go test -race -covermode=atomic -coverprofile=coverage.out ./internal/... ./cmd/... && go tool cover -html=coverage.out
    make lint     # golangci-lint run
    make vuln     # go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
    make e2e      # go test -race ./e2e/...
    make all      # lint + vuln + test + e2e

`-race` requires a C toolchain in `PATH` (gcc/clang/mingw-w64). CI runners provide this; Windows developers may need to install one locally.

CI uses `go test -race -covermode=atomic -coverprofile=coverage.out ./...` (single pass, all packages).

---

## GitHub Automation

### Dependabot (`.github/dependabot.yml`)

Auto-create weekly dependency update PRs for both Go modules and GitHub Actions.

### CI (`.github/workflows/ci.yml`)

Triggered on push and pull_request. Declares minimum permissions explicitly (`contents: read`) so the workflow never inherits more than it needs.

Install tools explicitly before use:
- `golangci-lint` via `golangci/golangci-lint-action` (use the official action, not `go install`)
- `govulncheck` via `go install golang.org/x/vuln/cmd/govulncheck@latest`

Runs: `go test -race -covermode=atomic -coverprofile=coverage.out ./...`, coverage upload to Codecov, `golangci-lint run` (pinned version), `govulncheck ./...`, and a smoke-test of the native linux/amd64 build (`./envlens --version`).

### Commitlint (`.github/workflows/commitlint.yml`)

Triggered on pull_request (opened, edited, synchronize). Declares minimum permissions explicitly (`pull-requests: read`).
Validates that the PR title follows Conventional Commits format (`feat:`, `fix:`, `chore:`, etc.).
Required for Release Please to correctly determine version bumps.

### Release Please

Requires three files. The workflow needs elevated permissions to create PRs and push tags:

    # .github/workflows/release-please.yml
    permissions:
      contents: write
      pull-requests: write

Config files with exact contents:

    # release-please-config.json
    {
      "packages": {
        ".": { "release-type": "go" }
      }
    }

    # .release-please-manifest.json
    { ".": "0.1.0" }

Merging the Release PR creates a `v*` tag and triggers the GoReleaser workflow.

### GoReleaser (`.github/workflows/release.yml` + `.goreleaser.yaml`)

Triggered on `v*` tag push. Needs write permission to publish releases:

    # .github/workflows/release.yml
    permissions:
      contents: write

Performs:
- Cross-build: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- Archives: tar.gz (Linux/macOS), zip (Windows), with `LICENSE` and `README.md` included in each archive
- Reproducible builds via `-trimpath` and embedded version:
  ```
  builds:
    - flags:
        - -trimpath
      ldflags:
        - -s -w
        - -X main.version={{.Version}}
  ```
- Checksum file generation
- Publish to GitHub Releases

### Codecov (`codecov.yml`)

Blocks PRs when coverage drops more than 5% compared to the base branch.

---

## Testing Strategy

### testdata/ Layout

Go tests run with the **package directory** as the working directory.
Each package must manage its own `testdata/` independently — they cannot share one.

    internal/parser/testdata/   # fixtures for parser unit tests
    internal/validate/testdata/ # optional; validate is table-driven today, add fixtures only when a rule genuinely needs one
    e2e/testdata/               # .env fixtures for E2E tests + compiled binary (gitignored)

### Unit Tests (`internal/*/xxx_test.go`)

Validate each package in isolation. No mocking required since there are no external I/O dependencies.

- `internal/parser`: Feed various format strings and assert on parsed key-value results
- `internal/diff`: Pass key maps directly and assert on diff output
- `internal/validate`: Feed valid and invalid values per rule and assert on errors
- `internal/display`: Capture `io.Writer` output and assert on rendered strings

Always use fixture files from the package's own `testdata/` rather than inline string literals.

### E2E Tests (`e2e/e2e_test.go`)

Execute the compiled binary via `exec.Command` and assert on stdout, stderr, and exit code.
No external services are required since envlens only reads local files.
Fixture `.env` files live in `e2e/testdata/`.

Use `TestMain` to build the binary and store the path in a package-level variable.
Resolve the binary name once to avoid repeating the `runtime.GOOS` check in every test:

    var binaryPath string

    func TestMain(m *testing.M) {
        binaryPath = "testdata/envlens"
        if runtime.GOOS == "windows" {
            binaryPath += ".exe"
        }
        cmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/envlens")
        if err := cmd.Run(); err != nil {
            log.Fatalf("failed to build binary: %v", err)
        }
        os.Exit(m.Run())
    }

    func TestDiffMissingKey(t *testing.T) {
        cmd := exec.Command(binaryPath, "diff", "testdata/a.env", "testdata/b.env")
        out, _ := cmd.Output()
        assert.Equal(t, 1, cmd.ProcessState.ExitCode())
        assert.Contains(t, string(out), "STRIPE_SECRET_KEY")
    }

### Style

- Use Go's standard table-driven test format: `[]struct{ name, input, want }`
- Name subtests descriptively: `TestParseQuotedValue/double_quoted`
- Mark test helpers with `t.Helper()`

### Coverage Targets

| Package | Target |
|---|---|
| `internal/parser` | 90%+ |
| `internal/diff` | 85%+ |
| `internal/validate` | 85%+ |
| `internal/display` | 70%+ |
| E2E | All commands and major flags covered |

### Edge Cases to Cover

- Empty file
- Comment-only file
- Duplicate keys (last value wins; a warning is emitted to stderr)
- Keys with no value (`KEY=`)
- Quoted empty value (`KEY=""`)
- Inline comment adjacent to value
- Multibyte characters in values
- Windows line endings (`\r\n`)
- UTF-8 BOM at file start

---

## Community Files

### `CONTRIBUTING.md`

Must include:

- Required tools: Go 1.22+, golangci-lint, govulncheck
- Setup: fork → clone → `make build` → `make all`
- Commit format: Conventional Commits required (`feat:`, `fix:`, `docs:`, `chore:`, etc.)
- PR checklist: `make all` passes, tests added/updated, README updated if needed

### `SECURITY.md`

Must include:

- How to report: use GitHub private vulnerability reporting (not a public issue)
- What to include: description, reproduction steps, affected versions, potential impact
- Expected response time

### Issue Templates (`.github/ISSUE_TEMPLATE/`)

- `bug_report.md`: steps to reproduce, expected vs actual behavior, `envlens --version`, OS and Go version
- `feature_request.md`: problem description, proposed solution, alternatives considered

### PR Template (`.github/pull_request_template.md`)

Must include:

- Summary of changes
- Type of change (bug fix / feature / docs / chore)
- Checklist: `make all` passes, tests added/updated, README updated if needed
- Reminder that the PR title must follow Conventional Commits format

---

## Hierarchical CLAUDE.md

Sub-packages may have their own `CLAUDE.md` files following this structure:

    # {Package Name}
    ## Overview
    One-line description.
    ## Key Types
    List of exported types and their purpose.
    ## Testing Notes
    Coverage target and what to mock/stub.

---

## References

- Global guidelines: `~/.claude/CLAUDE.md` (loaded before this file every session)
- dotenv format reference: https://hexdocs.pm/dotenvy/dotenv-file-format.html
