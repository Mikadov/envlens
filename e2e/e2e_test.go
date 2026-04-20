package e2e_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// binaryPath is the path to the compiled envlens binary built by TestMain.
// It is relative to the package directory, since Go tests run with that as
// the working directory.
var binaryPath string

func TestMain(m *testing.M) {
	binaryPath = "testdata/envlens"
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/envlens")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to build envlens binary: %v", err)
	}
	os.Exit(m.Run())
}

// runBinary runs envlens with the given args and returns stdout, stderr, and
// exit code.
func runBinary(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Disable ANSI color so assertions are stable.
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("run %s: %v", binaryPath, err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestVersion(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "--version")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "envlens")
}

func TestHelp_NoArgs(t *testing.T) {
	t.Parallel()
	_, stderr, code := runBinary(t)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "Usage:")
}

func TestDiff_DifferencesFound(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "diff", "testdata/a.env", "testdata/b.env")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "STRIPE_SECRET_KEY")
	assert.Contains(t, out, "MAIL_FROM_ADDRESS")
	assert.Contains(t, out, "OLD_API_KEY")
}

func TestDiff_Clean(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "diff", "testdata/a.env", "testdata/a.env")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "No differences found.")
}

func TestDiff_ValuesFlagMasksSecrets(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "diff", "--values", "testdata/a.env", "testdata/b.env")
	assert.Equal(t, 1, code)
	// Sensitive key masked even with --values.
	assert.Contains(t, out, "STRIPE_SECRET_KEY = ****")
	assert.NotContains(t, out, "sk_test_xxx")
	// Non-sensitive values still shown (MAIL_FROM_ADDRESS is non-secret).
	assert.Contains(t, out, "admin@example.com")
}

func TestDiff_Quiet(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "diff", "--quiet", "testdata/a.env", "testdata/b.env")
	assert.Equal(t, 1, code)
	assert.Empty(t, out)
}

func TestDiff_FileNotFound(t *testing.T) {
	t.Parallel()
	_, stderr, code := runBinary(t, "diff", "testdata/does_not_exist.env", "testdata/b.env")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "envlens:")
}

func TestValidate_Clean(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "validate", "testdata/valid.env")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "No errors found.")
}

func TestValidate_Errors(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "validate", "testdata/invalid.env")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "APP_URL")
	assert.Contains(t, out, "DB_PORT")
	assert.Contains(t, out, "APP_KEY")
	assert.Contains(t, out, "FROM_EMAIL")
	assert.Contains(t, out, "APP_DEBUG")
	assert.Contains(t, out, "error(s) found")
}

func TestValidate_StrictListsUnmatched(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "validate", "--strict", "testdata/a.env")
	// a.env has APP_NAME (no rule) and DB_HOST (no rule); others are valid.
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "APP_NAME")
	assert.Contains(t, out, "DB_HOST")
	assert.Contains(t, out, "no rule defined")
}

func TestShow_MasksByDefault(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "show", "testdata/show.env")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "****")
	assert.NotContains(t, out, "super-secret")
	assert.NotContains(t, out, "hunter2")
	assert.Contains(t, out, "Total: 4 keys")
}

func TestShow_NoMask(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "show", "--no-mask", "testdata/show.env")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "super-secret")
	assert.Contains(t, out, "hunter2")
}

func TestShow_JSONIsValidAndMasks(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "show", "--json", "testdata/show.env")
	assert.Equal(t, 0, code)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	assert.Equal(t, "MyApp", parsed["APP_NAME"])
	assert.Equal(t, "****", parsed["APP_KEY"])
	assert.Equal(t, "****", parsed["DB_PASSWORD"])
}

func TestShow_JSONNoMask(t *testing.T) {
	t.Parallel()
	out, _, code := runBinary(t, "show", "--json", "--no-mask", "testdata/show.env")
	assert.Equal(t, 0, code)
	var parsed map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	assert.Equal(t, "super-secret", parsed["APP_KEY"])
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	_, stderr, code := runBinary(t, "nope")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "unknown command")
}

func TestSubcommand_Help(t *testing.T) {
	t.Parallel()
	for _, sub := range []string{"diff", "validate", "show"} {
		sub := sub
		t.Run(sub, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, code := runBinary(t, sub, "--help")
			assert.Equal(t, 0, code, "%s --help should exit 0", sub)
			assert.Contains(t, stdout, "Usage of "+sub)
			assert.Empty(t, stderr, "help output must not leak into stderr")
		})
	}
}

func TestSubcommand_BadFlag(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runBinary(t, "diff", "--bogus")
	assert.Equal(t, 2, code)
	assert.Empty(t, stdout, "parse errors must not leak into stdout")
	assert.Contains(t, stderr, "not defined")
}

// runBinary sets NO_COLOR=1; this test confirms the binary honors it and
// emits no ANSI escapes on stdout. The name covers the combined guarantee:
// piped / non-TTY output plus the NO_COLOR env variable together.
func TestDiff_NoAnsiWhenNoColorSet(t *testing.T) {
	t.Parallel()
	out, _, _ := runBinary(t, "diff", "testdata/a.env", "testdata/b.env")
	assert.False(t, strings.Contains(out, "\x1b["), "stdout should not contain ANSI escapes")
}
