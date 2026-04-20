package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestRun_NoArgs(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "Usage:")
}

func TestRun_Version(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"--version", "-v", "version"} {
		arg := arg
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := run([]string{"envlens", arg}, &stdout, &stderr)
			assert.Equal(t, exitOK, code)
			assert.Contains(t, stdout.String(), "envlens")
			assert.Empty(t, stderr.String())
		})
	}
}

func TestRun_VersionStringWithVPrefix(t *testing.T) {
	// Not parallel: mutates the package-level `version` variable, which
	// other (parallel) tests read via run() → versionString().
	orig := version
	t.Cleanup(func() { version = orig })
	version = "1.2.3"
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "--version"}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Equal(t, "envlens v1.2.3\n", stdout.String())
}

func TestRun_Help(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "--help"}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "Usage:")
}

func TestRun_UnknownCommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "blorp"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "unknown command")
}

func TestRun_Diff_Clean(t *testing.T) {
	t.Parallel()
	a := writeFixture(t, "a.env", "APP_NAME=MyApp\nDB_HOST=localhost\n")
	b := writeFixture(t, "b.env", "APP_NAME=MyApp\nDB_HOST=localhost\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", a, b}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "No differences found.")
}

func TestRun_Diff_Differences(t *testing.T) {
	t.Parallel()
	a := writeFixture(t, "a.env", "APP_NAME=MyApp\nOLD=bye\n")
	b := writeFixture(t, "b.env", "APP_NAME=MyApp\nSTRIPE_SECRET_KEY=sk_test\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", a, b}, &stdout, &stderr)
	assert.Equal(t, exitDiff, code)
	assert.Contains(t, stdout.String(), "STRIPE_SECRET_KEY")
	assert.Contains(t, stdout.String(), "OLD")
}

func TestRun_Diff_WithValues_MasksSecrets(t *testing.T) {
	t.Parallel()
	a := writeFixture(t, "a.env", "APP_NAME=Real\n")
	b := writeFixture(t, "b.env", "APP_NAME=Demo\nAPP_KEY=supersecret\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", "--values", a, b}, &stdout, &stderr)
	assert.Equal(t, exitDiff, code)
	assert.Contains(t, stdout.String(), "****")
	assert.NotContains(t, stdout.String(), "supersecret")
}

func TestRun_Diff_Quiet(t *testing.T) {
	t.Parallel()
	a := writeFixture(t, "a.env", "APP_NAME=Real\n")
	b := writeFixture(t, "b.env", "STRIPE_SECRET_KEY=sk_test\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", "--quiet", a, b}, &stdout, &stderr)
	assert.Equal(t, exitDiff, code)
	assert.Empty(t, stdout.String())
}

func TestRun_Diff_WrongArgCount(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", "only_one.env"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "exactly two")
}

func TestRun_Diff_FileNotFound(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", "does_not_exist.env", "also_missing.env"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "envlens:")
}

func TestRun_Validate_Clean(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env",
		"APP_URL=https://example.com\nDB_PORT=5432\nAPP_KEY=secret\n",
	)
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "validate", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "No errors found.")
	assert.Contains(t, stdout.String(), "APP_URL")
}

func TestRun_Validate_Errors(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env",
		"APP_URL=http://\nDB_PORT=abc\nAPP_KEY=ok\n",
	)
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "validate", path}, &stdout, &stderr)
	assert.Equal(t, exitDiff, code)
	assert.Contains(t, stdout.String(), "2 error(s) found")
	assert.Contains(t, stdout.String(), "APP_URL")
	assert.Contains(t, stdout.String(), "DB_PORT")
}

func TestRun_Validate_StrictShowsUnmatched(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_URL=https://example.com\nDB_HOST=localhost\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "validate", "--strict", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "DB_HOST")
	assert.Contains(t, stdout.String(), "no rule defined")
}

func TestRun_Validate_SkipsUnmatchedByDefault(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_URL=https://example.com\nDB_HOST=localhost\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "validate", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "APP_URL")
	assert.NotContains(t, stdout.String(), "DB_HOST")
}

func TestRun_Validate_WrongArgCount(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "validate"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "exactly one")
}

func TestRun_Show_MasksByDefault(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_NAME=MyApp\nAPP_KEY=secret\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "show", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "****")
	assert.NotContains(t, stdout.String(), "secret")
	assert.Contains(t, stdout.String(), "Total: 2 keys")
}

func TestRun_Show_NoMask(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_KEY=revealed\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "show", "--no-mask", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stdout.String(), "revealed")
}

func TestRun_Show_JSON(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_NAME=MyApp\nAPP_KEY=secret\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "show", "--json", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(stdout.String()), "{"))
	assert.Contains(t, stdout.String(), `"APP_KEY": "****"`)
}

func TestRun_Show_WrongArgCount(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "show"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "exactly one")
}

func TestRun_Show_FileNotFound(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "show", "does_not_exist.env"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
}

func TestRun_Diff_BadFlag(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", "--bogus"}, &stdout, &stderr)
	assert.Equal(t, exitUsage, code)
	assert.Contains(t, stderr.String(), "not defined")
	assert.Empty(t, stdout.String(), "parse errors must not bleed into stdout")
}

func TestRun_Subcommand_Help(t *testing.T) {
	t.Parallel()
	cases := []string{"diff", "validate", "show"}
	for _, sub := range cases {
		sub := sub
		t.Run(sub, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := run([]string{"envlens", sub, "--help"}, &stdout, &stderr)
			assert.Equal(t, exitOK, code, "%s --help should exit 0", sub)
			assert.Contains(t, stdout.String(), "Usage of "+sub)
			assert.Empty(t, stderr.String(), "help output must go to stdout")
		})
	}
}

func TestRun_Diff_WarningsToStderr(t *testing.T) {
	t.Parallel()
	a := writeFixture(t, "a.env", "APP_NAME=MyApp\nDUP=1\nDUP=2\n")
	b := writeFixture(t, "b.env", "APP_NAME=MyApp\nDUP=2\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "diff", a, b}, &stdout, &stderr)
	assert.Equal(t, exitOK, code, "diff should be clean (DUP folds to 2)")
	assert.Contains(t, stderr.String(), `duplicate key "DUP"`)
	assert.Contains(t, stderr.String(), a)
}

func TestRun_Validate_WarningsToStderr(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_URL=https://example.com\n1BAD=x\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "validate", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stderr.String(), "invalid key")
}

func TestRun_Show_WarningsToStderr(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ".env", "APP_NAME=MyApp\nBROKEN=\"unmatched\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"envlens", "show", path}, &stdout, &stderr)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, stderr.String(), "unmatched")
	// show still succeeded for the valid entry.
	assert.Contains(t, stdout.String(), "APP_NAME")
}

func TestShouldColor_RespectsNoColor(t *testing.T) {
	t.Parallel()
	assert.False(t, shouldColor(&bytes.Buffer{}, true))
}

func TestShouldColor_NonFileWriter(t *testing.T) {
	t.Parallel()
	// A bytes.Buffer is not *os.File; color must be disabled.
	assert.False(t, shouldColor(&bytes.Buffer{}, false))
}
