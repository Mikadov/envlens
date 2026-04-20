# Contributing to envlens

Thanks for your interest! envlens aims to be a small, dependency-free,
portfolio-grade Go CLI, so contributions that keep that bar high are very
welcome.

## Required tools

- Go **1.22** or later
- [golangci-lint](https://golangci-lint.run/usage/install/)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
  (installed on demand by `make vuln`)
- A C toolchain on the `PATH` (gcc, clang, or mingw-w64 on Windows) —
  required by `go test -race`, which `make test`, `make e2e`, and
  `make cover` all use. CI runners have this preinstalled.

## Getting started

```bash
# 1. Fork the repo on GitHub, then clone your fork
git clone https://github.com/<your-username>/envlens.git
cd envlens

# 2. Verify a clean build
make build

# 3. Run the full check suite
make all
```

`make all` runs `lint`, `vuln`, `test`, and `e2e`. Every contribution must
leave this green.

## Development loop

- `make test` — unit tests for `internal/...` and `cmd/...`
- `make e2e` — end-to-end tests against the compiled binary
- `make cover` — open an HTML coverage report
- `make lint` — golangci-lint

Per-package coverage targets (see `CLAUDE.md`):

| Package | Target |
|---|---|
| `internal/parser`   | 90%+ |
| `internal/diff`     | 85%+ |
| `internal/validate` | 85%+ |
| `internal/display`  | 70%+ |

## Commit messages

envlens uses [Conventional Commits](https://www.conventionalcommits.org/) so
that [Release Please](https://github.com/googleapis/release-please) can
determine version bumps from merged PR titles.

Use one of:

- `feat:` — a new feature (minor bump)
- `fix:` — a bug fix (patch bump)
- `docs:` — documentation only
- `chore:` — tooling, CI, dependency bumps
- `test:` — test-only changes
- `refactor:` — refactoring without behavioural change
- `perf:` — performance improvements

Breaking changes: add `!` after the type (e.g., `feat!: redesign the diff output`)
or add a `BREAKING CHANGE:` footer.

## Pull requests

Before opening a PR, please confirm:

- [ ] `make all` passes locally
- [ ] Tests were added or updated
- [ ] README was updated if the public surface changed
- [ ] PR title follows Conventional Commits (the CI will enforce this)

Keep changes focused. One logical improvement per PR makes review easy and
keeps the changelog readable.

## Adding a validation rule

1. Add the rule to `Rules` in [`internal/validate/rules.go`](internal/validate/rules.go)
2. Add positive and negative cases in `rules_test.go`
3. Document the new suffix in `README.md` and `CLAUDE.md`
