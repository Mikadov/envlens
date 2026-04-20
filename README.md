<div align="center">
  <h1>envlens</h1>
  <p><strong>Env</strong>ironment <strong>Lens</strong></p>

  [![CI](https://github.com/Mikadov/envlens/actions/workflows/ci.yml/badge.svg)](https://github.com/Mikadov/envlens/actions/workflows/ci.yml)
  [![codecov](https://codecov.io/gh/Mikadov/envlens/branch/main/graph/badge.svg)](https://codecov.io/gh/Mikadov/envlens)
  [![Go Report Card](https://goreportcard.com/badge/github.com/Mikadov/envlens)](https://goreportcard.com/report/github.com/Mikadov/envlens)
  [![Go Reference](https://pkg.go.dev/badge/github.com/Mikadov/envlens.svg)](https://pkg.go.dev/github.com/Mikadov/envlens)
  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
</div>

A small, zero-dependency Go CLI for the everyday dotenv hygiene tasks:
**compare** two `.env` files, **validate** values against rules inferred from
key names, and **display** secrets without leaking them into your terminal
scrollback. Drops straight into any CI pipeline as a single static binary.

## Installation

<details open>
<summary><strong>Using <code>go install</code> (recommended)</strong></summary>

```bash
go install github.com/Mikadov/envlens/cmd/envlens@latest
envlens --version
```

</details>

<details>
<summary>Pre-built binaries (GitHub Releases)</summary>

Grab the archive for your platform from the
[Releases page](https://github.com/Mikadov/envlens/releases). Each release
ships with a `checksums.txt` you can verify against.

```bash
export VERSION=0.1.0
export OS=linux   # linux | darwin | windows
export ARCH=amd64 # amd64 | arm64

curl -LO "https://github.com/Mikadov/envlens/releases/download/v${VERSION}/envlens_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf "envlens_${VERSION}_${OS}_${ARCH}.tar.gz"
./envlens --version
```

> [!TIP]
> Windows releases are distributed as `.zip`. All archives include `LICENSE`
> and `README.md`.

</details>

<details>
<summary>From source</summary>

```bash
git clone https://github.com/Mikadov/envlens.git
cd envlens
make build          # produces ./bin/envlens
./bin/envlens --version
```

Requires Go **1.22** or later.

</details>

## Getting Started

### Keep `.env` and `.env.example` in sync

The most common use: compare your local `.env` against the project's
tracked `.env.example` to catch missing or stray keys.

```text
$ envlens diff .env .env.example
Missing in .env (defined in .env.example):
  - STRIPE_SECRET_KEY

Extra in .env (not in .env.example):
  - OLD_API_KEY
```

Exit code `1` on any difference, `0` on a clean match — so this is the one
command you want wired into CI. Add `--quiet` if you want a silent
pass/fail.

### Validate values, not just keys

`validate` infers rules from the key name. A port must be numeric; a
scheme-qualified URL must also have a host; a secret must be non-empty
— and so on.

```text
$ envlens validate .env
✗ APP_URL  not a valid URL (got "http://")
✗ APP_KEY  must not be empty
✗ DB_PORT  not a valid port number (got "abc")

3 error(s) found.
```

Keys without a matching rule are skipped silently. Pass `--strict` to
surface them as `?  KEY  no rule defined`.

### Inspect a `.env` safely

`cat .env` is a decent way to leak secrets into your terminal scrollback.
`show` renders the file as a table with sensitive values masked:

```text
$ envlens show .env
┌─────────────┬───────────┐
│ KEY         │ VALUE     │
├─────────────┼───────────┤
│ APP_NAME    │ MyApp     │
│ APP_KEY     │ ****      │
│ DB_HOST     │ 127.0.0.1 │
│ DB_PASSWORD │ ****      │
└─────────────┴───────────┘

Total: 4 keys
```

Sensitive keys are those whose name matches `_KEY`, `_SECRET`, `_TOKEN`,
or `_PASSWORD`. `--no-mask` unhides everything; `--json` emits
machine-readable output (masking still applied).

## Commands

### `envlens diff <file1> <file2>`

Compare **keys** between two `.env` files. Exit code **1** on any
difference, **0** on a clean match. Add `--values` to also compare values;
secrets remain masked as `****`.

```text
$ envlens diff .env .env.example
Missing in .env (defined in .env.example):
  - MAIL_FROM_ADDRESS
  - STRIPE_SECRET_KEY

Extra in .env (not in .env.example):
  - OLD_API_KEY
```

With `--values`, values are shown inline; sensitive keys are masked from
the same rule table used by `show`:

```text
$ envlens diff --values .env .env.example
Missing in .env (defined in .env.example):
  - MAIL_FROM_ADDRESS = admin@example.com
  - STRIPE_SECRET_KEY = ****

Extra in .env (not in .env.example):
  - OLD_API_KEY = ****
```

| Flag | Description |
|---|---|
| `--values`   | Include value differences (masked for sensitive keys) |
| `--quiet`    | Suppress all output; rely on the exit code only |
| `--no-color` | Disable ANSI colour output |

> [!NOTE]
> Colour output is enabled automatically when stdout is a TTY. When piping
> to a file or tool (e.g., `| tee`), output is plain text. See
> [Environment variables](#environment-variables) for overrides.

### `envlens validate <file>`

Check values against rules inferred from each key's name. Keys without a
matching rule are skipped silently (pass `--strict` to list them).

```text
$ envlens validate .env
✗ APP_URL  not a valid URL (got "http://")
✗ APP_KEY  must not be empty
✗ DB_PORT  not a valid port number (got "abc")
✗ FROM_EMAIL  not a valid email address (got "notanemail")
✗ APP_DEBUG  must be one of true/false/1/0 (got "maybe")

5 error(s) found.
```

A clean file:

```text
$ envlens validate .env
✓ APP_URL  OK
✓ APP_KEY  OK
✓ DB_PORT  OK
✓ FROM_EMAIL  OK
✓ APP_DEBUG  OK

No errors found.
```

With `--strict`, keys with no matching rule are surfaced as `?`:

```text
$ envlens validate --strict .env
? APP_NAME  no rule defined
✓ APP_URL  OK
? DB_HOST  no rule defined
✓ DB_PORT  OK
```

| Flag | Description |
|---|---|
| `--strict`   | Also print keys with no matching rule |
| `--no-color` | Disable ANSI colour output |

See [Validation rules](#validation-rules) for the full suffix table.

### `envlens show <file>`

A safer alternative to `cat .env`. Sensitive values are masked as `****`
by default; use `--no-mask` to unhide, or `--json` for machine-readable
output.

```text
$ envlens show .env
┌─────────────┬───────────┐
│ KEY         │ VALUE     │
├─────────────┼───────────┤
│ APP_NAME    │ MyApp     │
│ APP_KEY     │ ****      │
│ DB_HOST     │ 127.0.0.1 │
│ DB_PASSWORD │ ****      │
└─────────────┴───────────┘

Total: 4 keys
```

JSON output (masking still applied unless `--no-mask`):

```json
$ envlens show --json .env
{
  "APP_NAME": "MyApp",
  "APP_KEY": "****",
  "DB_HOST": "127.0.0.1",
  "DB_PASSWORD": "****"
}
```

| Flag | Description |
|---|---|
| `--no-mask` | Display all values without masking (use with care) |
| `--json`    | Emit JSON instead of a table (keys preserve file order) |

> [!WARNING]
> `--no-mask` prints secrets to stdout. Avoid in logs, screenshares, or CI
> output — anything it produces can end up in scrollback or job archives.

> [!NOTE]
> Masking is triggered by key **name**, not value. A secret stored under a
> key that does not match a sensitive suffix (e.g., `API_TOKENS_JSON`)
> will be displayed verbatim. Rename the key or extend the rule table.

### `envlens --version`

```text
$ envlens --version
envlens v0.1.0
```

Development builds (plain `go build` without GoReleaser) print
`envlens dev`.

## Exit codes

Exit codes are the CI-integration contract. They are stable across
versions.

| Code | Meaning |
|------|---------|
| `0` | Success. No differences / no validation errors. |
| `1` | `diff` found differences, or `validate` found one or more errors. |
| `2` | Usage error (bad flag, wrong arg count) or I/O error (missing file). |

> [!TIP]
> Pair `envlens diff --quiet` with `|| exit 1` in shell scripts when you
> want the first failure to short-circuit, without any stdout noise.

## Validation rules

Rules are matched by **key name suffix**, case-insensitive:

| Suffix                                        | Validation                                |
|-----------------------------------------------|-------------------------------------------|
| `_URL`, `_URI`                                | Non-empty and whitespace-free. Scheme-qualified values (`scheme://…`) must have a scheme; `file://` URIs may have an empty host, other schemes require one. Bare values like `localhost`, `localhost:3000`, `example.com`, or `127.0.0.1:5432` are accepted. |
| `_PORT`                                       | Integer in range 1–65535                  |
| `_KEY`, `_SECRET`, `_TOKEN`, `_PASSWORD`      | Non-empty; value is also **masked**       |
| `_EMAIL`, `_ADDRESS`                          | Valid email address                       |
| `_ENABLED`, `_DEBUG`, `_FLAG`                 | One of `true`, `false`, `1`, `0`          |

Keys that don't match any suffix are skipped silently by default. Pass
`--strict` to surface them as `?  KEY  no rule defined`.

> [!NOTE]
> Adding a new rule is a five-line change in
> [`internal/validate/rules.go`](internal/validate/rules.go); see the
> "Adding a validation rule" section of [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Environment variables

envlens itself reads very few env vars — it only cares about how the
terminal wants to be treated:

| Variable     | Effect |
|--------------|--------|
| `NO_COLOR`   | Any non-empty value disables ANSI colour output (honours the [NO_COLOR](https://no-color.org/) standard). |
| `TERM=dumb`  | Also disables ANSI colour output.                                                                          |

Colour is additionally suppressed automatically when stdout is not a TTY
(e.g., when piping into `tee`, redirecting to a file, or running inside
most CI log viewers). You can always force it off with `--no-color`.

## CI integration

Drop envlens into any pipeline that can run a single binary. Example
GitHub Actions workflow:

```yaml
name: env-check
on: [push, pull_request]

jobs:
  envlens:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/Mikadov/envlens/cmd/envlens@latest

      - name: Keys in .env.example must match the CI env
        run: envlens diff .env.example .env.ci

      - name: Values in .env.ci must be well-formed
        run: envlens validate .env.ci
```

> [!TIP]
> The exit code is the contract. You don't need any custom `--format` or
> JSON post-processing to wire envlens into a CI that understands
> non-zero exits — which is all of them.

## Parser specification (v1)

The parser is hand-written on top of `bufio` + `strings`, with no
external dependencies. It deliberately stays close to the subset of
`.env` syntax that most dotenv dialects agree on.

### Supported

```bash
# a comment line
KEY=value
KEY="double quoted value"
KEY='single quoted value'
KEY=                        # empty value (allowed)
KEY=value  # inline comment (requires whitespace before #)
export KEY=value            # export prefix is stripped
```

Values that contain `#` without preceding whitespace are treated as
literal, which is usually what you want:

```bash
COLOR=#ff0000                # value = "#ff0000"
URL=http://host/page#frag    # value = "http://host/page#frag"
```

Duplicate keys are accepted; the **last occurrence wins**. A warning is
printed to stderr (along with any unmatched-quote, missing-`=`, or
invalid-key warnings) and does not affect the exit code.

### Cross-platform nuances

- **UTF-8 BOM** at the start of the file is stripped transparently.
- **Windows line endings** (`\r\n`) are handled.
- **Multibyte values** (CJK, emoji) are preserved; the table renderer
  approximates East Asian width so alignment is usually correct, with
  rare drift on exotic grapheme clusters.

## Limitations

Deliberately out of scope for v1 — documented here so there are no
surprises:

| Not supported                                   | What envlens does instead                    |
|-------------------------------------------------|----------------------------------------------|
| Variable interpolation (`${OTHER_KEY}`)         | Value is kept as literal text (no expansion) |
| Multi-line values (backslash or newline quote)  | Line is skipped with a warning               |
| Nested or escaped quotes                        | Line is skipped with a warning               |
| `.env` variant resolution (`.env.local` chain)  | Compare individual files with `diff`         |
| User-defined schema                             | Use key name suffixes (rule table)           |

Variant resolution and custom schemas are the most commonly requested
additions and are tracked for a future release.

## Troubleshooting

<details>
<summary>My secret key is not being masked.</summary>

Masking is triggered by key **name**, not value. Only keys whose name ends
with `_KEY`, `_SECRET`, `_TOKEN`, or `_PASSWORD` (case-insensitive) are
masked. Rename the variable (e.g., `FOO_SECRET` rather than
`FOO_CREDENTIALS`) or add a new suffix in
[`internal/validate/rules.go`](internal/validate/rules.go).

</details>

<details>
<summary>Output has garbled <code>\x1b[31m</code> escape sequences.</summary>

Your terminal or log viewer is not interpreting ANSI colour codes. Either
set `NO_COLOR=1`, `TERM=dumb`, or pass `--no-color`. CI log viewers
usually strip colour correctly; this typically shows up in older Windows
consoles or in saved log files.

</details>

<details>
<summary>Table columns look misaligned for CJK / emoji values.</summary>

envlens approximates East Asian width using a stdlib-only heuristic
(see `runeWidth` in
[`internal/display/display.go`](internal/display/display.go)). Rare
grapheme clusters and exotic emoji may drift one column. For strict
alignment, pipe through `column -t -s '│'` or use `--json`.

</details>

<details>
<summary><code>envlens --version</code> prints <code>envlens dev</code>.</summary>

You are running a development build (`go build` without ldflags). Release
archives from GitHub Releases and binaries from
`go install ...@v0.1.0` print the tagged version. See
[`.goreleaser.yaml`](.goreleaser.yaml) for the build recipe.

</details>

<details>
<summary>Line 7 of my <code>.env</code> is being skipped with a warning.</summary>

v1 does not support multi-line values, variable interpolation, or nested
quotes. The parser skips the offending line and continues — it never
panics. Reshape the value (e.g., JSON-encode it onto a single line) or
wait for a release that supports the construct you need.

</details>

## Development

```bash
make build   # build ./bin/envlens
make test    # unit tests (internal + cmd)
make e2e     # end-to-end tests against the compiled binary
make cover   # HTML coverage report
make lint    # golangci-lint
make vuln    # govulncheck
make all     # lint + vuln + test + e2e
```

Further reading:

- [`CONTRIBUTING.md`](CONTRIBUTING.md) — commit message conventions, PR
  checklist, and how to add a validation rule.
- [`SECURITY.md`](SECURITY.md) — vulnerability reporting.
- [`CLAUDE.md`](CLAUDE.md) — architecture notes, test strategy, and
  coverage targets.

## License

[MIT](LICENSE) &copy; 2026 Mikadov
